package storage

import "testing"

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
