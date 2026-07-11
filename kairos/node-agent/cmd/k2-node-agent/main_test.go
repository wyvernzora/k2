package main

import "testing"

func TestRunRejectsDeletedModeFlag(t *testing.T) {
	if err := run([]string{"setup-persistence", "--mode", "required", "--verify-only"}); err == nil {
		t.Fatal("expected unknown mode flag error")
	}
}

func TestRunRejectsDeletedVerifyPrefixFlag(t *testing.T) {
	if err := run([]string{"setup-persistence", "--verify-prefix", "/dev/nvme", "--verify-only"}); err == nil {
		t.Fatal("expected unknown verify-prefix flag error")
	}
}

func TestRunRejectsOldSaveconfigFlag(t *testing.T) {
	if err := run([]string{"storage-health", "--saveconfig", "/tmp/saveconfig.json"}); err == nil {
		t.Fatal("expected unknown saveconfig flag error")
	}
}
