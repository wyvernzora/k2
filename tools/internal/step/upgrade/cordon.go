package upgrade

import "context"

// Cordon marks the kubernetes node unschedulable.
func (r *Runner) Cordon(ctx context.Context, plan Plan) error {
	return r.Kubectl.Cordon(ctx, plan.NodeName)
}
