package main

import "testing"

func TestRunRejectsInvalidMode(t *testing.T) {
	if err := run([]string{"storage", "--mode", "wat", "--verify-only"}); err == nil {
		t.Fatal("expected invalid mode error")
	}
}
