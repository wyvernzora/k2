package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeFakeFakeFakeFakeFakeFakeFakeFakeFakeFake operator"

func TestLoadDeduplicatesLiteralAndFileKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "operator.keys")
	if err := os.WriteFile(path, []byte("# comment\n\n"+testKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load([]string{testKey}, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != testKey {
		t.Fatalf("keys = %#v", got)
	}
}

func TestLoadRejectsGitHubKeys(t *testing.T) {
	_, err := Load([]string{"github:wyvernzora"}, nil)
	if err == nil || !strings.Contains(err.Error(), "github:") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsMissingKeys(t *testing.T) {
	_, err := Load(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("error = %v", err)
	}
}
