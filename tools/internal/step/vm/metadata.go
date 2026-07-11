package vm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func root(repoRoot string) string {
	return filepath.Join(repoRoot, ".testvm")
}

func dir(repoRoot string, id string) string {
	return filepath.Join(root(repoRoot), "vm-"+id)
}

func normalizeID(value string) string {
	value = filepath.Base(value)
	value = strings.TrimPrefix(value, "vm-")
	value = strings.TrimPrefix(value, "k2-qemu-")
	return value
}

func metadataPath(repoRoot string, id string) string {
	return filepath.Join(dir(repoRoot, normalizeID(id)), "vm.json")
}

func loadMetadata(repoRoot string, id string) (Metadata, error) {
	data, err := os.ReadFile(metadataPath(repoRoot, id))
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	// Back-fill fields added after the metadata file was first written.
	// Older VMs don't have ConsoleSocket persisted; derive it so callers
	// have a stable path to use once the VM is restarted under the new
	// QEMU args. The VM still needs a restart for the socket to actually
	// be created — QEMU args are baked at boot time.
	if meta.ConsoleSocket == "" && meta.VMDir != "" {
		meta.ConsoleSocket = filepath.Join(meta.VMDir, "console.sock")
	}
	return meta, nil
}

func writeMetadata(meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(meta.VMDir, "vm.json"), data, 0o644)
}

func listMetadata(repoRoot string) ([]Metadata, error) {
	entries, err := os.ReadDir(root(repoRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var metas []Metadata
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "vm-") {
			continue
		}
		meta, err := loadMetadata(repoRoot, strings.TrimPrefix(entry.Name(), "vm-"))
		if err != nil {
			continue
		}
		if meta.Backend == "qemu" {
			metas = append(metas, meta)
		}
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].ID < metas[j].ID })
	return metas, nil
}
