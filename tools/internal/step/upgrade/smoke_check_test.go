package upgrade

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForSmokeCheckRetriesUntilSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		err := waitForSmokeCheck(context.Background(), time.Minute, 5*time.Second, func(context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("node is not Ready")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("waitForSmokeCheck: %v", err)
		}
		if attempts != 3 {
			t.Fatalf("attempts = %d, want 3", attempts)
		}
	})
}

func TestWaitForSmokeCheckTimeoutIncludesLastError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		err := waitForSmokeCheck(context.Background(), 11*time.Second, 5*time.Second, func(context.Context) error {
			attempts++
			return errors.New("node is not Ready")
		})
		if err == nil {
			t.Fatal("expected timeout")
		}
		if !strings.Contains(err.Error(), "timed out after 11s") || !strings.Contains(err.Error(), "node is not Ready") {
			t.Fatalf("error = %q, want timeout and last smoke-check error", err)
		}
		if attempts != 3 {
			t.Fatalf("attempts = %d, want 3", attempts)
		}
	})
}

func TestWaitForSmokeCheckHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := waitForSmokeCheck(ctx, time.Minute, 5*time.Second, func(context.Context) error {
		attempts++
		cancel()
		return errors.New("node is not Ready")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
