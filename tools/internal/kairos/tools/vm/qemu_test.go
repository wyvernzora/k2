package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQEMUArgsAttachExtraDisksAfterPersistent(t *testing.T) {
	dir := t.TempDir()
	meta := Metadata{
		ID:              "storage",
		Name:            "k2-qemu-storage",
		VMDir:           dir,
		KairosQCOW2:     filepath.Join(dir, "kairos.qcow2"),
		PersistentQCOW2: filepath.Join(dir, "persistent.qcow2"),
		ExtraDisks: []Disk{
			{ID: "extra-storage-1", QCOW2: filepath.Join(dir, "extra-storage-1.qcow2"), SizeMB: 4096},
			{ID: "extra-storage-2", QCOW2: filepath.Join(dir, "extra-storage-2.qcow2"), SizeMB: 4096},
		},
		QGAPort:       25000,
		MonitorPort:   24000,
		ConsoleLog:    filepath.Join(dir, "console.log"),
		ConsoleSocket: filepath.Join(dir, "console.sock"),
		NetworkMode:   "vmnet-shared",
		MemoryMB:      4096,
		CPUs:          2,
	}
	for _, path := range []string{meta.PersistentQCOW2, meta.ExtraDisks[0].QCOW2, meta.ExtraDisks[1].QCOW2} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	args := strings.Join(qemuArgs(meta, "/firmware.fd", "vmnet-shared,id=net0"), "\n")
	persistent := strings.Index(args, "id=persistent")
	extra1 := strings.Index(args, "id=extra-storage-1")
	extra2 := strings.Index(args, "id=extra-storage-2")
	if persistent < 0 || extra1 < 0 || extra2 < 0 {
		t.Fatalf("qemu args missing expected disk ids:\n%s", args)
	}
	if !(persistent < extra1 && extra1 < extra2) {
		t.Fatalf("disk order persistent=%d extra1=%d extra2=%d args:\n%s", persistent, extra1, extra2, args)
	}
}
