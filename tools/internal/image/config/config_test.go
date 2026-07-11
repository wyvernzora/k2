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

func TestLoadOverlayMetadataAcceptsAptPurge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	if err := os.WriteFile(path, []byte("build:\n  aptPurge:\n    - neovim\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadOverlayMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Build.AptPurge) != 1 || got.Build.AptPurge[0] != "neovim" {
		t.Fatalf("aptPurge = %#v, want neovim", got.Build.AptPurge)
	}
}
