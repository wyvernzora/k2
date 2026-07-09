package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func listPresets(repoRoot string) ([]Preset, error) {
	entries, err := os.ReadDir(presetsDir(repoRoot))
	if err != nil {
		return nil, err
	}
	var presets []Preset
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		preset, err := loadPreset(repoRoot, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		presets = append(presets, preset)
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].name < presets[j].name })
	return presets, nil
}

func loadPreset(repoRoot string, name string) (Preset, error) {
	path := filepath.Join(presetsDir(repoRoot), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Preset{}, fmt.Errorf("read VM preset %s: %w", path, err)
	}
	var preset Preset
	if err := json.Unmarshal(data, &preset); err != nil {
		return Preset{}, fmt.Errorf("decode VM preset %s: %w", path, err)
	}
	preset.name = name
	return preset, nil
}

func resolvePreset(repoRoot string, name string) (Preset, string, error) {
	preset, err := loadPreset(repoRoot, name)
	if err != nil {
		return Preset{}, "", err
	}
	if preset.MemoryMB == 0 {
		preset.MemoryMB = 4096
	}
	if preset.CPUs == 0 {
		preset.CPUs = 2
	}
	if preset.PersistentDisk.SizeMB == 0 {
		preset.PersistentDisk.SizeMB = 8192
	}
	if preset.Network.Mode == "" {
		preset.Network.Mode = "user"
	}
	target := preset.Target
	switch target {
	case "host-qemu", "":
		target, err = defaultQEMUTarget()
		if err != nil {
			return Preset{}, "", err
		}
	case "host-qemu-storage":
		target, err = defaultQEMUStorageTarget()
		if err != nil {
			return Preset{}, "", err
		}
	}
	return preset, target, nil
}

func ResolvePresetArtifactTarget(repoRoot string, name string) (string, error) {
	_, target, err := resolvePreset(repoRoot, name)
	return target, err
}

func presetsDir(repoRoot string) string {
	return filepath.Join(repoRoot, "kairos", "tools", "vm-presets")
}

func defaultQEMUTarget() (string, error) {
	switch runtime.GOARCH {
	case "arm64":
		return "ubuntu-26.04-arm64-qemu-k8s", nil
	case "amd64":
		return "ubuntu-26.04-amd64-qemu-k8s", nil
	default:
		return "", fmt.Errorf("unsupported host architecture for default QEMU target: %s", runtime.GOARCH)
	}
}

func defaultQEMUStorageTarget() (string, error) {
	switch runtime.GOARCH {
	case "arm64":
		return "ubuntu-26.04-arm64-qemu-storage", nil
	case "amd64":
		return "ubuntu-26.04-amd64-qemu-storage", nil
	default:
		return "", fmt.Errorf("unsupported host architecture for default QEMU storage target: %s", runtime.GOARCH)
	}
}
