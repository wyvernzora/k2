package textfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path atomically because node_exporter may read it at any
// point while a oneshot collector is refreshing it.
func Write(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary textfile: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary textfile: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary textfile mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary textfile: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace textfile: %w", err)
	}
	return nil
}
