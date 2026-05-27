package flash

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// ErrRpibootNotInstalled signals that the `rpiboot` binary isn't on
// PATH. The CLI surfaces this with an install hint.
var ErrRpibootNotInstalled = errors.New("flash: `rpiboot` binary not found on PATH (install via `brew install rpiboot` on macOS or from https://github.com/raspberrypi/usbboot)")

// ErrRpibootTimeout signals that no new disks appeared within the
// configured window after `rpiboot` exited.
var ErrRpibootTimeout = errors.New("flash: timed out waiting for rpiboot-attached disks to appear")

// RunRpiboot invokes the `rpiboot` binary with its stdout/stderr wired
// through `out` so the operator sees rpiboot's own messages. Returns
// ErrRpibootNotInstalled if the binary isn't on PATH.
func RunRpiboot(ctx context.Context, out io.Writer) error {
	path, err := exec.LookPath("rpiboot")
	if err != nil {
		return ErrRpibootNotInstalled
	}
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flash: rpiboot exited %w", err)
	}
	return nil
}

// WaitForNewDisks polls the Platform once per second until the diff
// against `before` produces a non-empty set that stays stable for one
// extra poll, or until `timeout` elapses. The stabilize window
// prevents racing the second of two disks (eMMC + NVMe) when only one
// has shown up so far.
//
// Returns the new disks (post-diff) on success, or ErrRpibootTimeout
// when nothing new appears in time.
func WaitForNewDisks(ctx context.Context, p Platform, before []Disk, timeout time.Duration) ([]Disk, error) {
	deadline := time.Now().Add(timeout)
	const pollInterval = 1 * time.Second

	var lastDiff []Disk
	lastDiffCount := -1
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		after, err := p.EnumerateExternalDisks(ctx)
		if err != nil {
			return nil, fmt.Errorf("flash: enumerate disks while waiting: %w", err)
		}
		diff := DiffDisks(before, after)
		if len(diff) > 0 && len(diff) == lastDiffCount {
			// Same set of new disks for two polls in a row → settled.
			return diff, nil
		}
		lastDiff = diff
		lastDiffCount = len(diff)
		if time.Now().After(deadline) {
			if len(lastDiff) > 0 {
				return lastDiff, nil
			}
			return nil, ErrRpibootTimeout
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
