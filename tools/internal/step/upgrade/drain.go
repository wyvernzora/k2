package upgrade

import (
	"context"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
)

// Drain evicts pods off the node. Streams kubectl's eviction log
// through the configured Kubectl.Stdout (the caller wires this to a
// ui.Step's writer).
func (r *Runner) Drain(ctx context.Context, plan Plan, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultDrainTimeout
	}
	return r.Kubectl.Drain(ctx, plan.NodeName, kubectl.DrainOpts{
		Timeout:            timeout,
		IgnoreDaemonsets:   true,
		DeleteEmptyDirData: true,
		GracePeriodSeconds: -1,
	})
}
