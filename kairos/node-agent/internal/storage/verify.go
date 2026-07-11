package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

var stateDir = "/usr/local/.state"

func (m manager) verify() error {
	src, _ := m.run.Output("findmnt", "-n", "-o", "SOURCE", "/usr/local")
	resolved := src
	if src != "" {
		if out, err := m.run.Output("readlink", "-f", src); err == nil && out != "" {
			resolved = out
		}
	}
	m.log.Printf("/usr/local mounted from %s resolved %s", src, resolved)
	if resolved == "" {
		return fmt.Errorf("/usr/local is not mounted from required %s storage", persistLabel)
	}
	srcDisk, err := diskForDev(resolved, m.run)
	if err != nil {
		return fmt.Errorf("resolve /usr/local source disk: %w", err)
	}
	bootDisk, err := m.bootDisk()
	if err != nil {
		m.log.Printf("could not detect boot disk during verify: %v", err)
	} else if srcDisk == bootDisk {
		return fmt.Errorf("/usr/local source %q resolved %q is on boot disk %s", src, resolved, bootDisk)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, filepath.Base(m.cfg.Marker)), []byte("ok\n"), 0o644)
}
