package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRepoRoot(start string) (string, error) {
	explicit := start != ""
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	original := abs
	for {
		if isRepoRoot(abs) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			if explicit {
				return original, nil
			}
			return "", fmt.Errorf("could not find repo root from %s", start)
		}
		abs = parent
	}
}

func isRepoRoot(path string) bool {
	return exists(filepath.Join(path, "tools", "go.mod")) &&
		exists(filepath.Join(path, "package.json")) &&
		exists(filepath.Join(path, "apps")) &&
		exists(filepath.Join(path, "clusters")) &&
		exists(filepath.Join(path, "kairos"))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
