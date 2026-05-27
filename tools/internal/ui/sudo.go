package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const sudoKeepAliveInterval = 60 * time.Second

func newSudoCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "sudo", args...)
}

func newTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// Sudo caches the operator's sudo credentials up front.
func (r *Reporter) Sudo(ctx context.Context, reason string) (release func(), err error) {
	if r == nil {
		return func() {}, fmt.Errorf("nil reporter")
	}
	r.Infof("Caching sudo: %s", reason)

	cmd := newSudoCommand(ctx, "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return func() {}, fmt.Errorf("sudo authentication: %w", err)
	}
	r.Successf("sudo authenticated")

	keepAliveCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := newTicker(sudoKeepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-ticker.C:
				_ = newSudoCommand(keepAliveCtx, "-n", "-v").Run()
			}
		}
	}()
	return cancel, nil
}
