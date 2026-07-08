package storage

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirstPartition(t *testing.T) {
	tests := map[string]string{
		"/dev/nvme0n1": "/dev/nvme0n1p1",
		"/dev/mmcblk0": "/dev/mmcblk0p1",
		"/dev/vda":     "/dev/vda1",
		"/dev/sda":     "/dev/sda1",
		"/dev/loop0":   "/dev/loop0p1",
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

func TestTargetDiskRejectsBootDiskAlias(t *testing.T) {
	root := t.TempDir()
	restore := setStorageRootsForTest(t, root)
	defer restore()
	oldStat := statBlock
	statBlock = func(string) (fs.FileInfo, error) { return fakeBlockInfo{}, nil }
	t.Cleanup(func() { statBlock = oldStat })

	if err := os.MkdirAll(filepath.Join(devRoot, "disk", "by-id"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devRoot, "sda"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(devRoot, "disk", "by-id", "boot")
	if err := os.Symlink(filepath.Join(devRoot, "sda"), alias); err != nil {
		t.Fatal(err)
	}
	run := &fakeRunner{outputFunc: func(name string, args ...string) (string, error) {
		key := name + " " + strings.Join(args, " ")
		switch key {
		case "findmnt -n -o SOURCE /run/initramfs/cos-state":
			return "", nil
		case "findmnt -n -o SOURCE /oem":
			return filepath.Join(devRoot, "sda2"), nil
		case "readlink -f " + filepath.Join(devRoot, "sda"):
			return filepath.Join(devRoot, "sda"), nil
		case "readlink -f " + filepath.Join(devRoot, "sda2"):
			return filepath.Join(devRoot, "sda2"), nil
		case "lsblk -no PKNAME " + filepath.Join(devRoot, "sda"):
			return "sda", nil
		case "lsblk -no PKNAME " + filepath.Join(devRoot, "sda2"):
			return "sda", nil
		default:
			if name == "readlink" && len(args) == 2 && args[0] == "-f" && filepath.Base(args[1]) == "sda" {
				return args[1], nil
			}
			if name == "lsblk" && len(args) == 3 && args[0] == "-no" && args[1] == "PKNAME" && filepath.Base(args[2]) == "sda" {
				return "sda", nil
			}
			return "", nil
		}
	}}
	m := manager{cfg: Config{Disk: alias, WaitSeconds: 0}, run: run, log: logger{prefix: "test"}}

	_, err := m.targetDisk()
	if err == nil {
		t.Fatal("expected boot disk alias rejection")
	}
	if !strings.Contains(err.Error(), "resolves to boot disk") {
		t.Fatalf("error = %v", err)
	}
}

func TestTargetDiskExplicitFailsWhenBootDiskUnknown(t *testing.T) {
	root := t.TempDir()
	restore := setStorageRootsForTest(t, root)
	defer restore()
	oldStat := statBlock
	statBlock = func(string) (fs.FileInfo, error) { return fakeBlockInfo{}, nil }
	t.Cleanup(func() { statBlock = oldStat })

	disk := filepath.Join(devRoot, "sdb")
	if err := os.WriteFile(disk, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m := manager{cfg: Config{Disk: disk, WaitSeconds: 0}, run: &fakeRunner{}, log: logger{prefix: "test"}}

	_, err := m.targetDisk()
	if err == nil {
		t.Fatal("expected boot disk detection error")
	}
	if !strings.Contains(err.Error(), "detect boot disk") {
		t.Fatalf("error = %v", err)
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

func TestDevicesByLabelDistinguishesBlkidExitCodes(t *testing.T) {
	exit2 := exec.Command("sh", "-c", "exit 2").Run()
	got, err := devicesByLabel("COS_PERSISTENT", &fakeRunner{outputFunc: func(string, ...string) (string, error) {
		return "", exit2
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("devices = %#v, want empty", got)
	}

	exit1 := exec.Command("sh", "-c", "exit 1").Run()
	_, err = devicesByLabel("COS_PERSISTENT", &fakeRunner{outputFunc: func(string, ...string) (string, error) {
		return "", exit1
	}})
	if err == nil {
		t.Fatal("expected non-exit-2 blkid error")
	}
}

func TestPrepareTargetDiskLabelsNewPersistentBeforeOld(t *testing.T) {
	root := t.TempDir()
	restore := setStorageRootsForTest(t, root)
	defer restore()
	oldStat := statBlock
	statBlock = func(string) (fs.FileInfo, error) { return fakeBlockInfo{}, nil }
	t.Cleanup(func() { statBlock = oldStat })

	labelCalls := 0
	newPersistentLabeled := false
	run := &fakeRunner{
		runFunc: func(name string, args ...string) error {
			if name == "e2label" && strings.Join(args, " ") == "/dev/sdb1 COS_PERSISTENT" {
				newPersistentLabeled = true
			}
			return nil
		},
		outputFunc: func(name string, args ...string) (string, error) {
			key := name + " " + strings.Join(args, " ")
			switch key {
			case "blkid -o device -t LABEL=COS_PERSISTENT":
				labelCalls++
				if labelCalls == 1 || !newPersistentLabeled {
					return "/dev/sda1", nil
				}
				return "/dev/sda1\n/dev/sdb1", nil
			case "readlink -f /dev/sda1":
				return "/dev/sda1", nil
			case "readlink -f /dev/sdb1":
				return "/dev/sdb1", nil
			case "lsblk -no PKNAME /dev/sda1":
				return "sda", nil
			case "lsblk -no PKNAME /dev/sdb1":
				return "sdb", nil
			case "findmnt -rn --source /dev/sda1 -o TARGET":
				return "", nil
			case "findmnt -rn --source /dev/sdb1":
				return "/mnt/new", nil
			case "blkid -s TYPE -o value /dev/sdb1", "blkid -s TYPE -o value /dev/sda1":
				return "ext4", nil
			default:
				return "", nil
			}
		},
	}
	m := manager{cfg: Config{WaitSeconds: 0, OldLabel: "COS_PERSIST_OLD"}, run: run, log: logger{prefix: "test"}}

	if err := m.prepareTargetDisk("/dev/sdb"); err != nil {
		t.Fatal(err)
	}
	newLabel := run.indexOfRun("e2label /dev/sdb1 COS_PERSISTENT")
	oldLabel := run.indexOfRun("e2label /dev/sda1 COS_PERSIST_OLD")
	if newLabel == -1 || oldLabel == -1 {
		t.Fatalf("runs missing expected labels: %#v", run.runKeys)
	}
	if newLabel > oldLabel {
		t.Fatalf("old label ran before new label: %#v", run.runKeys)
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
	runKeys    []string
}

func (r *fakeRunner) Run(name string, args ...string) error {
	r.runs++
	r.runKeys = append(r.runKeys, name+" "+strings.Join(args, " "))
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

func (r *fakeRunner) indexOfRun(want string) int {
	for i, key := range r.runKeys {
		if key == want {
			return i
		}
	}
	return -1
}

func setStorageRootsForTest(t *testing.T, root string) func() {
	t.Helper()
	oldSysRoot, oldDevRoot := sysRoot, devRoot
	sysRoot = root
	devRoot = filepath.Join(root, "dev")
	if err := os.MkdirAll(devRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	return func() {
		sysRoot = oldSysRoot
		devRoot = oldDevRoot
	}
}

type fakeBlockInfo struct{}

func (fakeBlockInfo) Name() string       { return "block" }
func (fakeBlockInfo) Size() int64        { return 0 }
func (fakeBlockInfo) Mode() fs.FileMode  { return os.ModeDevice }
func (fakeBlockInfo) ModTime() time.Time { return time.Time{} }
func (fakeBlockInfo) IsDir() bool        { return false }
func (fakeBlockInfo) Sys() any           { return nil }
