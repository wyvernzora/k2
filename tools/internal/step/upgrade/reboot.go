package upgrade

import (
	"context"
	"fmt"
	"time"
)

// Reboot triggers a reboot and waits for SSH to come back up.
// `timeout` caps the wait; on expiry we return a wrapped timeout
// error and the caller decides whether the node is broken or just
// slow.
func (r *Runner) Reboot(ctx context.Context, plan Plan, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultRebootTimeout
	}
	if err := r.Remote.RunAllowDisconnect("sudo reboot"); err != nil {
		return fmt.Errorf("reboot command: %w", err)
	}
	// Give the node a moment to actually drop the SSH session + kill
	// sshd before we start probing. Otherwise our first probe finds
	// the OLD sshd still alive and we declare the reboot "instant".
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
	}
	if err := r.Remote.WaitForAuthCtx(ctx, timeout); err != nil {
		return fmt.Errorf("wait for SSH after reboot: %w", err)
	}
	return nil
}
