package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m manager) verify() error {
	src, _ := m.run.Output("findmnt", "-n", "-o", "SOURCE", "/usr/local")
	resolved := src
	if src != "" {
		if out, err := m.run.Output("readlink", "-f", src); err == nil && out != "" {
			resolved = out
		}
	}
	m.log.Printf("/usr/local mounted from %s resolved %s", src, resolved)
	if m.cfg.VerifyPrefix != "" && !strings.HasPrefix(resolved, m.cfg.VerifyPrefix) {
		return fmt.Errorf("/usr/local source %q resolved %q does not match %q", src, resolved, m.cfg.VerifyPrefix)
	}
	stateDir := "/usr/local/.state"
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, filepath.Base(m.cfg.Marker)), []byte("ok\n"), 0o644)
}
