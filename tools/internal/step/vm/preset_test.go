package vm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePresetHostQEMUStorage(t *testing.T) {
	root := t.TempDir()
	presetDir := filepath.Join(root, "kairos", "tools", "vm-presets")
	if err := os.MkdirAll(presetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
  "target": "host-qemu-storage",
  "network": {"mode": "vmnet-shared"},
  "persistentDisk": {"enabled": true},
  "extraDisks": {"count": 2, "sizeMb": 4096}
}`
	if err := os.WriteFile(filepath.Join(presetDir, "storage.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	preset, target, err := resolvePreset(root, "storage")
	if err != nil {
		t.Fatal(err)
	}
	want := "ubuntu-26.04-" + runtime.GOARCH + "-qemu-storage"
	if target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}
	if preset.ExtraDisks.Count != 2 || preset.ExtraDisks.SizeMB != 4096 {
		t.Fatalf("extra disks = %#v, want count=2 size=4096", preset.ExtraDisks)
	}
	if preset.PersistentDisk.SizeMB != 8192 {
		t.Fatalf("persistent disk size = %d, want default 8192", preset.PersistentDisk.SizeMB)
	}
}

func TestResolveExtraDisksOverride(t *testing.T) {
	preset := Preset{ExtraDisks: ExtraDisks{Count: 2, SizeMB: 4096}}
	got, err := resolveExtraDisks(preset, CreateOptions{ExtraDisks: 1, ExtraDiskSizeMB: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.SizeMB != 1024 {
		t.Fatalf("extra disks = %#v, want count=1 size=1024", got)
	}

	got, err = resolveExtraDisks(preset, CreateOptions{ExtraDiskSizeMB: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 2 || got.SizeMB != 8192 {
		t.Fatalf("extra disk size override = %#v, want count=2 size=8192", got)
	}

	_, err = resolveExtraDisks(Preset{}, CreateOptions{ExtraDisks: 1})
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("error = %v, want missing size error", err)
	}
}

func TestExtraDisksForVMMetadata(t *testing.T) {
	got := extraDisksForVM("abc123", "/tmp/vm-abc123", ExtraDisks{Count: 2, SizeMB: 4096})
	if len(got) != 2 {
		t.Fatalf("disk count = %d, want 2", len(got))
	}
	if got[0].ID != "extra-abc123-1" ||
		got[0].QCOW2 != "/tmp/vm-abc123/extra-abc123-1.qcow2" ||
		got[0].SizeMB != 4096 ||
		got[1].ID != "extra-abc123-2" ||
		got[1].QCOW2 != "/tmp/vm-abc123/extra-abc123-2.qcow2" ||
		got[1].SizeMB != 4096 {
		t.Fatalf("extra disk metadata = %#v", got)
	}
}
