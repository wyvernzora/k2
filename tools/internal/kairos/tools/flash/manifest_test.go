package flash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestReadsRpi4cbShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact-manifest.json")
	writeFile(t, path, `{
  "target": "ubuntu-24.04-standard-arm64-rpi4cb-k3s",
  "kairosVersion": "v4.1.0",
  "k3sVersion": "v1.36.0+k3s1",
  "raw": {
    "file": "image.raw",
    "sha256": "d87037a2",
    "sizeBytes": 3586129920
  },
  "compressed": {
    "file": "image.raw.xz",
    "sha256": "93cd9b33",
    "sizeBytes": 1053758740
  },
  "patches": {}
}`)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Target != "ubuntu-24.04-standard-arm64-rpi4cb-k3s" {
		t.Fatalf("target = %s", m.Target)
	}
	if m.Raw.File != "image.raw" || m.Raw.SHA256 != "d87037a2" || m.Raw.SizeBytes != 3586129920 {
		t.Fatalf("raw = %+v", m.Raw)
	}
	if m.Compressed.File != "image.raw.xz" || m.Compressed.SHA256 != "93cd9b33" || m.Compressed.SizeBytes != 1053758740 {
		t.Fatalf("compressed = %+v", m.Compressed)
	}
}

func TestLoadManifestMissingFieldErrors(t *testing.T) {
	cases := map[string]string{
		"target":          `{"raw":{"file":"f","sha256":"h","sizeBytes":1},"compressed":{"file":"c"}}`,
		"raw.file":        `{"target":"t","raw":{"sha256":"h","sizeBytes":1},"compressed":{"file":"c"}}`,
		"raw.sha256":      `{"target":"t","raw":{"file":"f","sizeBytes":1},"compressed":{"file":"c"}}`,
		"raw.sizeBytes":   `{"target":"t","raw":{"file":"f","sha256":"h"},"compressed":{"file":"c"}}`,
		"compressed.file": `{"target":"t","raw":{"file":"f","sha256":"h","sizeBytes":1}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "m.json")
			writeFile(t, path, body)
			if _, err := LoadManifest(path); err == nil {
				t.Fatalf("expected error for missing %s", name)
			} else if !strings.Contains(err.Error(), name) {
				t.Fatalf("error %v does not mention missing field %s", err, name)
			}
		})
	}
}

func TestLoadManifestBadJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.json")
	writeFile(t, path, `{ not json`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatalf("expected parse error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
