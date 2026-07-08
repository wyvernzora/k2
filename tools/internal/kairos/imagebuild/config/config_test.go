package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOverlayMetadataRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	if err := os.WriteFile(path, []byte("build:\n  aptPackage:\n    - qemu-guest-agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOverlayMetadata(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "aptPackage") {
		t.Fatalf("error = %v, want aptPackage", err)
	}
}
