package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
)

// SmokeCheck confirms the cluster-side post-reboot state: the node
// is Ready, no pods in non-Running/non-Succeeded phase are
// scheduled on it.
func (r *Runner) SmokeCheck(ctx context.Context, plan Plan) error {
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
