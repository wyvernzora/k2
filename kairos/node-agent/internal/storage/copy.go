package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m manager) copyPersistent(oldDev, newDev string) error {
	if oldDev == "" {
		m.log.Printf("no existing persistent filesystem to copy")
		return nil
	}

	work, err := os.MkdirTemp("", "k2-persist-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	oldMount := filepath.Join(work, "old")
	newMount := filepath.Join(work, "new")
	if err := os.MkdirAll(oldMount, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(newMount, 0o755); err != nil {
		return err
	}

	mountedOld := false
	existingMount, _ := m.run.Output("findmnt", "-rn", "--source", oldDev, "-o", "TARGET")
	if existingMount != "" {
		m.log.Printf("copying persistent data from existing mount %s", existingMount)
		oldMount = existingMount
	} else {
		m.log.Printf("copying persistent data from %s", oldDev)
		if err := m.run.Run("mount", "-o", "ro", oldDev, oldMount); err != nil {
			return fmt.Errorf("mount old persistent %s: %w", oldDev, err)
		}
		mountedOld = true
	}
	defer func() {
		if mountedOld {
			_ = m.run.Run("umount", oldMount)
		}
	}()

	if err := m.run.Run("mount", newDev, newMount); err != nil {
		return fmt.Errorf("mount new persistent %s: %w", newDev, err)
	}
	defer m.run.Run("umount", newMount)

	cmd := fmt.Sprintf("tar -C %s -cpf - . | tar -C %s -xpf -", shellQuote(oldMount), shellQuote(newMount))
	if err := m.run.Run("sh", "-c", cmd); err != nil {
		return fmt.Errorf("copy persistent data: %w", err)
	}
	_ = m.run.Run("sync")
	return nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
