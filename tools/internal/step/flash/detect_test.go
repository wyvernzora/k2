package flash

import (
	"errors"
	"testing"
)

const gib = uint64(1024 * 1024 * 1024)

func TestDiffDisksReturnsOnlyNewIDs(t *testing.T) {
	before := []Disk{{ID: "disk1"}, {ID: "disk2"}}
	after := []Disk{{ID: "disk1"}, {ID: "disk2"}, {ID: "disk4"}, {ID: "disk5"}}
	diff := DiffDisks(before, after)
	if len(diff) != 2 || diff[0].ID != "disk4" || diff[1].ID != "disk5" {
		t.Fatalf("diff = %+v", diff)
	}
}

func TestDiffDisksEmptyWhenNothingNew(t *testing.T) {
	before := []Disk{{ID: "disk1"}, {ID: "disk2"}}
	after := []Disk{{ID: "disk2"}, {ID: "disk1"}}
	diff := DiffDisks(before, after)
	if len(diff) != 0 {
		t.Fatalf("expected empty diff, got %+v", diff)
	}
}

func TestClassifyHappyPath(t *testing.T) {
	disks := []Disk{
		{ID: "disk4", Path: "/dev/disk4", SizeBytes: 32 * gib},  // eMMC
		{ID: "disk5", Path: "/dev/disk5", SizeBytes: 256 * gib}, // NVMe
	}
	got, err := Classify(disks)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if !got.HasEMMC || got.EMMC.ID != "disk4" {
		t.Fatalf("eMMC = %+v", got)
	}
	if !got.HasNVMe || got.NVMe.ID != "disk5" {
		t.Fatalf("NVMe = %+v", got)
	}
}

func TestClassifyEMMCOnly(t *testing.T) {
	disks := []Disk{{ID: "disk4", SizeBytes: 32 * gib}}
	got, err := Classify(disks)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if !got.HasEMMC {
		t.Fatalf("expected eMMC")
	}
	if got.HasNVMe {
		t.Fatalf("did not expect NVMe")
	}
}

func TestClassifyTwoEMMCSizedDisksAmbiguous(t *testing.T) {
	disks := []Disk{
		{ID: "disk4", Path: "/dev/disk4", SizeBytes: 32 * gib},
		{ID: "disk5", Path: "/dev/disk5", SizeBytes: 31 * gib},
	}
	_, err := Classify(disks)
	if !errors.Is(err, ErrAmbiguousDisks) {
		t.Fatalf("err = %v, want ErrAmbiguousDisks", err)
	}
}

func TestClassifyTwoNVMeSizedDisksAmbiguous(t *testing.T) {
	disks := []Disk{
		{ID: "disk5", Path: "/dev/disk5", SizeBytes: 256 * gib},
		{ID: "disk6", Path: "/dev/disk6", SizeBytes: 512 * gib},
	}
	_, err := Classify(disks)
	if !errors.Is(err, ErrAmbiguousDisks) {
		t.Fatalf("err = %v, want ErrAmbiguousDisks", err)
	}
}

func TestClassifyIgnoresOutOfBandDisks(t *testing.T) {
	disks := []Disk{
		{ID: "disk1", SizeBytes: 2 * gib},    // too small (USB stick / SD card)
		{ID: "disk2", SizeBytes: 4000 * gib}, // too big (external HDD)
		{ID: "disk4", SizeBytes: 32 * gib},   // eMMC
	}
	got, err := Classify(disks)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if !got.HasEMMC || got.EMMC.ID != "disk4" {
		t.Fatalf("eMMC = %+v", got)
	}
	if got.HasNVMe {
		t.Fatalf("did not expect NVMe")
	}
}
