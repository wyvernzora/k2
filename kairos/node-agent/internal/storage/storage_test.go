package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirstPartition(t *testing.T) {
	tests := map[string]string{
		"/dev/sdb":         "/dev/sdb1",
		"/dev/vdb":         "/dev/vdb1",
		"/dev/nvme0n1":     "/dev/nvme0n1p1",
		"/dev/mmcblk0":     "/dev/mmcblk0p1",
		"/dev/mmcblk0rpmb": "/dev/mmcblk0rpmbp1",
	}
	for disk, want := range tests {
		if got := FirstPartition(disk); got != want {
			t.Fatalf("FirstPartition(%q) = %q, want %q", disk, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cfg := Normalize(Config{})
	if cfg.Disk != "auto" {
		t.Fatalf("Disk = %q", cfg.Disk)
	}
	if cfg.OldLabel != "COS_PERSIST_OLD" {
		t.Fatalf("OldLabel = %q", cfg.OldLabel)
	}
	if cfg.WaitSeconds != 30 {
		t.Fatalf("WaitSeconds = %d", cfg.WaitSeconds)
	}
}

func TestParseMode(t *testing.T) {
	mode, err := ParseMode("required")
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Required() {
		t.Fatal("required mode parsed as optional")
	}

	mode, err = ParseMode("optional")
	if err != nil {
		t.Fatal(err)
	}
	if mode.Required() {
		t.Fatal("optional mode parsed as required")
	}

	if _, err := ParseMode("wat"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestTargetDiskAutoRetriesUntilDiskAppears(t *testing.T) {
	root := t.TempDir()
	oldSysRoot, oldDevRoot := sysRoot, devRoot
	oldRescanWait, oldSleep := rescanWait, sleepForRescan
	sysRoot = root
	devRoot = filepath.Join(root, "dev")
	rescanWait = time.Millisecond
	sleepForRescan = time.Sleep
	t.Cleanup(func() {
		sysRoot = oldSysRoot
		devRoot = oldDevRoot
		rescanWait = oldRescanWait
		sleepForRescan = oldSleep
	})

	if err := os.MkdirAll(filepath.Join(root, "sys", "block"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(devRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	run := &fakeRunner{
		runFunc: func(name string, args ...string) error {
			if name != "udevadm" || strings.Join(args, " ") != "settle" {
				return nil
			}
			if err := os.MkdirAll(filepath.Join(root, "sys", "block", "sdb"), 0o755); err != nil {
				return err
			}
			f, err := os.Create(filepath.Join(devRoot, "sdb"))
			if err != nil {
				return err
			}
			return f.Close()
		},
	}
	m := manager{cfg: Config{Disk: "auto", WaitSeconds: 1}, run: run, log: logger{prefix: "test"}}

	got, err := m.targetDisk()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(devRoot, "sdb")
	if got != want {
		t.Fatalf("targetDisk() = %q, want %q", got, want)
	}
	if run.runs < 1 {
		t.Fatal("expected udevadm settle to run")
	}
}

func TestVerifyRequiredRejectsUsrLocalOnBootDisk(t *testing.T) {
	run := &fakeRunner{
		outputFunc: func(name string, args ...string) (string, error) {
			key := name + " " + strings.Join(args, " ")
			switch key {
			case "findmnt -n -o SOURCE /usr/local":
				return "/dev/sda3", nil
			case "findmnt -n -o SOURCE /oem":
				return "/dev/sda2", nil
			case "readlink -f /dev/sda3":
				return "/dev/sda3", nil
			case "readlink -f /dev/sda2":
				return "/dev/sda2", nil
			case "lsblk -no PKNAME /dev/sda3":
				return "sda", nil
			case "lsblk -no PKNAME /dev/sda2":
				return "sda", nil
			default:
				return "", nil
			}
		},
	}
	m := manager{cfg: Config{Required: true}, run: run, log: logger{prefix: "test"}}

	err := m.verify()
	if err == nil {
		t.Fatal("expected boot disk verification error")
	}
	if !strings.Contains(err.Error(), "is on boot disk /dev/sda") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRequiredSkipsBootDiskCheckWhenBootDiskUnknown(t *testing.T) {
	oldStateDir := stateDir
	stateDir = t.TempDir()
	t.Cleanup(func() { stateDir = oldStateDir })

	run := &fakeRunner{
		outputFunc: func(name string, args ...string) (string, error) {
			key := name + " " + strings.Join(args, " ")
			switch key {
			case "findmnt -n -o SOURCE /usr/local":
				return "/dev/sdb1", nil
			case "readlink -f /dev/sdb1":
				return "/dev/sdb1", nil
			case "lsblk -no PKNAME /dev/sdb1":
				return "sdb", nil
			default:
				return "", nil
			}
		},
	}
	m := manager{cfg: Config{Required: true, Marker: ".marker"}, run: run, log: logger{prefix: "test"}}

	if err := m.verify(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, ".marker")); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
}

type fakeRunner struct {
	runFunc    func(name string, args ...string) error
	outputFunc func(name string, args ...string) (string, error)
	runs       int
}

func (r *fakeRunner) Run(name string, args ...string) error {
	r.runs++
	if r.runFunc != nil {
		return r.runFunc(name, args...)
	}
	return errors.New("not implemented")
}

func (r *fakeRunner) Output(name string, args ...string) (string, error) {
	if r.outputFunc != nil {
		return r.outputFunc(name, args...)
	}
	return "", nil
}
