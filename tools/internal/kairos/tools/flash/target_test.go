package flash

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRpi4cbTargetSingleMatch(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "ubuntu-24.04-standard-arm64-rpi4cb-k3s")

	got, err := ResolveRpi4cbTarget(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Manifest.Target != "ubuntu-24.04-standard-arm64-rpi4cb-k3s" {
		t.Fatalf("target = %s", got.Manifest.Target)
	}
	if !strings.HasSuffix(got.CompressedPath, "image.raw.xz") {
		t.Fatalf("compressed path = %s", got.CompressedPath)
	}
	if !strings.HasSuffix(got.RawPath, "image.raw") {
		t.Fatalf("raw path = %s", got.RawPath)
	}
}

func TestResolveRpi4cbTargetIgnoresNonRpi4cb(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "ubuntu-24.04-standard-arm64-qemu-k3s")
	writeManifest(t, root, "ubuntu-24.04-standard-amd64-qemu-k3s")

	_, err := ResolveRpi4cbTarget(root)
	if !errors.Is(err, ErrNoTargets) {
		t.Fatalf("err = %v, want ErrNoTargets", err)
	}
}

func TestResolveRpi4cbTargetMultipleMatchesAmbiguous(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "ubuntu-24.04-standard-arm64-rpi4cb-k3s")
	writeManifest(t, root, "alpine-3.20-standard-arm64-rpi4cb-k3s")

	_, err := ResolveRpi4cbTarget(root)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "multiple rpi4cb") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRpi4cbTargetSkipsUnparseableManifests(t *testing.T) {
	root := t.TempDir()
	badPath := filepath.Join(root, "kairos", "artifacts", "broken", "artifact-manifest.json")
	writeFile(t, badPath, "not json")
	writeManifest(t, root, "ubuntu-24.04-standard-arm64-rpi4cb-k3s")

	got, err := ResolveRpi4cbTarget(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Manifest.Target != "ubuntu-24.04-standard-arm64-rpi4cb-k3s" {
		t.Fatalf("target = %s", got.Manifest.Target)
	}
}

func writeManifest(t *testing.T, root, target string) {
	t.Helper()
	path := filepath.Join(root, "kairos", "artifacts", target, "artifact-manifest.json")
	body := `{
  "target": "` + target + `",
  "raw": {"file": "image.raw", "sha256": "abc", "sizeBytes": 1},
  "compressed": {"file": "image.raw.xz", "sha256": "def", "sizeBytes": 1}
}`
	writeFile(t, path, body)
}
