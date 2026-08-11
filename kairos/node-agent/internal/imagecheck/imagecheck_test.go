package imagecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlotTag(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{
			name: "storage",
			metadata: map[string]string{
				"flavor": "ubuntu-26.04", "arch": "amd64", "hardware": "qemu", "role": "storage",
			},
			want: "ubuntu-amd64-qemu-storage",
		},
		{
			name: "k8s",
			metadata: map[string]string{
				"flavor": "ubuntu-26.04", "arch": "arm64", "hardware": "rpi4cb", "role": "k8s",
			},
			want: "ubuntu-arm64-rpi4cb-k8s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := slotTag(tt.metadata)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("slotTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckDigestComparison(t *testing.T) {
	server := newRegistryServer(t, registryFixture{})
	tests := []struct {
		name          string
		appliedDigest string
		wantUpgrade   bool
	}{
		{name: "match", appliedDigest: "sha256:registry", wantUpgrade: false},
		{name: "mismatch", appliedDigest: "sha256:applied", wantUpgrade: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t, server, &tt.appliedDigest, "commit-a")
			got, err := check(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !got.success || got.upgradeAvailable != tt.wantUpgrade {
				t.Fatalf("check() = success %t, upgrade %t; want true, %t", got.success, got.upgradeAvailable, tt.wantUpgrade)
			}
			want := fmt.Sprintf("k2_node_image_upgrade_available %d\n", boolValue(tt.wantUpgrade))
			if body := render(got, 123); !strings.Contains(body, want) {
				t.Fatalf("rendered metrics missing %q:\n%s", want, body)
			}
		})
	}
}

func TestCheckCommitFallback(t *testing.T) {
	tests := []struct {
		name           string
		registryCommit string
		index          bool
		wantUpgrade    bool
	}{
		{name: "match", registryCommit: "commit-a", wantUpgrade: false},
		{name: "mismatch through OCI index", registryCommit: "commit-b", index: true, wantUpgrade: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newRegistryServer(t, registryFixture{revision: tt.registryCommit, index: tt.index})
			cfg := testConfig(t, server, nil, "commit-a")
			got, err := check(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !got.success || got.upgradeAvailable != tt.wantUpgrade {
				t.Fatalf("check() = success %t, upgrade %t; want true, %t", got.success, got.upgradeAvailable, tt.wantUpgrade)
			}
			body := render(got, 123)
			want := fmt.Sprintf("k2_node_image_upgrade_available %d\n", boolValue(tt.wantUpgrade))
			if !strings.Contains(body, want) || !strings.Contains(body, `applied_digest=""`) {
				t.Fatalf("rendered fallback metrics missing expected values:\n%s", body)
			}
		})
	}
}

func TestRunRegistryFailureWritesFailureMetricsWithoutUpgrade(t *testing.T) {
	server := newRegistryServer(t, registryFixture{manifestStatus: http.StatusInternalServerError})
	cfg := testConfig(t, server, nil, "commit-a")
	if err := Run(cfg); err == nil {
		t.Fatal("Run() error = nil, want registry failure")
	}

	body, err := os.ReadFile(filepath.Join(cfg.TextfileDir, textfileName))
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(body)
	if !strings.Contains(rendered, "k2_node_image_check_success 0\n") {
		t.Fatalf("failure metrics missing check_success 0:\n%s", rendered)
	}
	if !strings.Contains(rendered, "k2_node_image_check_timestamp_seconds ") {
		t.Fatalf("failure metrics missing timestamp:\n%s", rendered)
	}
	if strings.Contains(rendered, "k2_node_image_upgrade_available") {
		t.Fatalf("failure metrics contain upgrade_available:\n%s", rendered)
	}
	if _, err := os.Stat(filepath.Join(cfg.TextfileDir, "k2.prom")); !os.IsNotExist(err) {
		t.Fatalf("image check wrote metrics command textfile: %v", err)
	}
}

func TestCheckTokenRequestFailure(t *testing.T) {
	server := newRegistryServer(t, registryFixture{tokenStatus: http.StatusUnauthorized})
	cfg := testConfig(t, server, nil, "commit-a")
	if _, err := check(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "request registry token") {
		t.Fatalf("check() error = %v, want token request failure", err)
	}
}

type registryFixture struct {
	tokenStatus    int
	manifestStatus int
	digest         string
	revision       string
	index          bool
}

func newRegistryServer(t *testing.T, fixture registryFixture) *httptest.Server {
	t.Helper()
	if fixture.digest == "" {
		fixture.digest = "sha256:registry"
	}
	if fixture.revision == "" {
		fixture.revision = "commit-a"
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if r.Method != http.MethodGet {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			if got := r.URL.Query().Get("scope"); got != "repository:wyvernzora/k2-kairos:pull" {
				http.Error(w, "wrong scope", http.StatusBadRequest)
				return
			}
			if fixture.tokenStatus != 0 {
				http.Error(w, "token failed", fixture.tokenStatus)
				return
			}
			writeJSON(t, w, map[string]string{"token": "test-token"})
			return
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/manifests/") {
			for _, mediaType := range strings.Split(manifestAccept, ", ") {
				if !strings.Contains(r.Header.Get("Accept"), mediaType) {
					http.Error(w, "missing manifest accept type", http.StatusNotAcceptable)
					return
				}
			}
		}

		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/manifests/ubuntu-amd64-qemu-storage"):
			if fixture.manifestStatus != 0 {
				http.Error(w, "manifest failed", fixture.manifestStatus)
				return
			}
			w.Header().Set("Docker-Content-Digest", fixture.digest)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/manifests/ubuntu-amd64-qemu-storage"):
			if fixture.index {
				writeJSON(t, w, manifestDocument{Manifests: []descriptor{
					{Digest: "sha256:image", Platform: &platform{OS: "linux", Architecture: "amd64"}},
					{Digest: "sha256:provenance", Platform: &platform{OS: "unknown", Architecture: "unknown"}},
				}})
				return
			}
			writeJSON(t, w, manifestDocument{Config: descriptor{Digest: "sha256:config"}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/manifests/sha256:image"):
			writeJSON(t, w, manifestDocument{Config: descriptor{Digest: "sha256:config"}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blobs/sha256:config"):
			writeJSON(t, w, map[string]any{
				"config": map[string]any{
					"Labels": map[string]string{sourceCommitLabel: fixture.revision},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func testConfig(t *testing.T, server *httptest.Server, appliedDigest *string, sourceCommit string) Config {
	t.Helper()
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	metadata := "flavor: ubuntu-26.04\n" +
		"arch: amd64\n" +
		"hardware: qemu\n" +
		"role: storage\n" +
		"sourceCommit: " + sourceCommit + "\n"
	if err := os.WriteFile(metadataFile, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	digestFile := filepath.Join(t.TempDir(), "k2-image-digest")
	if appliedDigest != nil {
		if err := os.WriteFile(digestFile, []byte(*appliedDigest+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return normalize(Config{
		TextfileDir:  t.TempDir(),
		MetadataFile: metadataFile,
		DigestFile:   digestFile,
		HTTPClient:   server.Client(),
		TokenURL:     server.URL + "/token?scope=repository:wyvernzora/k2-kairos:pull",
		RegistryURL:  server.URL + "/v2/wyvernzora/k2-kairos",
	})
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
