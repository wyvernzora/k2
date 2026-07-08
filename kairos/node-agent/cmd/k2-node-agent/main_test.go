package main

import "testing"

func TestRunRejectsInvalidMode(t *testing.T) {
	if err := run([]string{"setup-persistence", "--mode", "wat", "--verify-only"}); err == nil {
		t.Fatal("expected invalid mode error")
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
