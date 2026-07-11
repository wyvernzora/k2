package build

import (
	"os"
	"path/filepath"
)

func PrepareDeployDirectory(repoRoot string) error {
	deployRoot := filepath.Join(repoRoot, "deploy")
	if err := os.RemoveAll(deployRoot); err != nil {
		return err
	}
	return os.MkdirAll(deployRoot, 0o755)
}
