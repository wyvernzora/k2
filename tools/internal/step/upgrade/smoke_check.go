package upgrade

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
)

const (
	defaultSmokeCheckTimeout = 5 * time.Minute
	smokeCheckPollInterval   = 5 * time.Second
)

// SmokeCheck confirms the cluster-side post-reboot state: the node
// is Ready, no pods in non-Running/non-Succeeded phase are
// scheduled on it. SSH becomes available before K3s necessarily
// updates the Node, so retry the complete check within a bounded
// post-reboot window.
func (r *Runner) SmokeCheck(ctx context.Context, plan Plan, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultSmokeCheckTimeout
	}
	return waitForSmokeCheck(ctx, timeout, smokeCheckPollInterval, func(ctx context.Context) error {
		return r.smokeCheckOnce(ctx, plan)
	})
}

func (r *Runner) smokeCheckOnce(ctx context.Context, plan Plan) error {
	nodes, err := r.Kubectl.Nodes(ctx)
	if err != nil {
		return err
	}
	var n kubectl.Node
	for _, candidate := range nodes {
		if candidate.Name == plan.NodeName {
			n = candidate
			break
		}
	}
	if n.Name == "" {
		return fmt.Errorf("node %s vanished from kubectl get nodes", plan.NodeName)
	}
	if !n.Ready() {
		return fmt.Errorf("node %s is not Ready after reboot", plan.NodeName)
	}
	bad, err := r.Kubectl.PodsOnNode(ctx, plan.NodeName, []string{"Running", "Succeeded"})
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		names := make([]string, len(bad))
		for i, p := range bad {
			names[i] = fmt.Sprintf("%s/%s(%s)", p.Namespace, p.Name, p.Phase)
		}
		return fmt.Errorf("%d non-Running pod(s) on %s: %s",
			len(bad), plan.NodeName, strings.Join(names, ", "))
	}
	return nil
}

func waitForSmokeCheck(
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	check func(context.Context) error,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := check(waitCtx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-waitCtx.Done():
			if err := ctx.Err(); err != nil {
				return err
			}
			return fmt.Errorf("smoke check timed out after %s: %w", timeout, lastErr)
		case <-ticker.C:
		}
	}
}
