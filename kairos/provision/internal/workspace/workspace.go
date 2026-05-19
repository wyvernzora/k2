package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRepoRoot(start string) (string, error) {
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
	for {
		if exists(filepath.Join(abs, "clusters")) && exists(filepath.Join(abs, "deploy")) && exists(filepath.Join(abs, "kairos")) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("could not find repo root from %s", start)
		}
		abs = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
