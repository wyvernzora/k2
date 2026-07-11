package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootUsesSourceMarkers(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"tools/go.mod", "package.json"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatalf("mkdir parent for %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte("test"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	for _, path := range []string{"apps", "clusters", "kairos", "apps/example/components"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}

	got, err := FindRepoRoot(filepath.Join(root, "apps", "example", "components"))
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindRepoRoot = %q, want %q", got, root)
	}
}

func TestFindRepoRootDoesNotRequireDeployOutput(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"tools/go.mod", "package.json"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatalf("mkdir parent for %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte("test"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	for _, path := range []string{"apps", "clusters", "kairos"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}

	if _, err := FindRepoRoot(root); err != nil {
		t.Fatalf("FindRepoRoot without deploy/: %v", err)
	}
}

func TestFindRepoRootTrustsExplicitPath(t *testing.T) {
	root := t.TempDir()
	got, err := FindRepoRoot(root)
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindRepoRoot = %q, want %q", got, root)
	}
}
