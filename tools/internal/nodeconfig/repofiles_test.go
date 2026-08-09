package nodeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Validates every checked-in node file so a typo fails in CI, not at
// provision time.
func TestCheckedInNodeFilesAreValid(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	targets, err := filepath.Glob(filepath.Join(repoRoot, "clusters", "*", "nodes", "*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) == 0 {
		t.Skip("no node files checked in")
	}
	for _, path := range targets {
		rel, _ := filepath.Rel(repoRoot, path)
		t.Run(rel, func(t *testing.T) {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			target, name := parts[1], strings.TrimSuffix(parts[3], ".toml")
			cfg, found, err := Load(repoRoot, target, name)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("glob found file but Load did not")
			}
			if len(cfg.NICs) > 0 && cfg.PrimaryAddress() == "" {
				t.Fatal("primary address unparseable")
			}
		})
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "clusters")); err != nil {
		t.Fatal("repo layout assumption broken:", err)
	}
}
