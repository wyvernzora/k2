package upgrade

import "context"

// Uncordon marks the kubernetes node schedulable again.
func (r *Runner) Uncordon(ctx context.Context, plan Plan) error {
	return r.Kubectl.Uncordon(ctx, plan.NodeName)
}
