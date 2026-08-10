package upgrade

import (
	"context"
	"fmt"
)

// UpgradeActive runs `sudo kairos-agent upgrade --source <ref>` on
// the node, which writes the new image to COS_ACTIVE. Does NOT
// reboot — Reboot is the next phase.
func (r *Runner) UpgradeActive(ctx context.Context, plan Plan) error {
	script := fmt.Sprintf("sudo kairos-agent upgrade --source %s", shellQuote(kairosUpgradeSource(plan.Target)))
	if err := r.Remote.Run(script); err != nil {
		return err
	}
	// Record what was actually installed. The image cannot bake its own
	// digest, so this marker on COS_PERSISTENT is how the next run answers
	// "is the tag still on the hash this node runs?" exactly rather than
	// falling back to the coarser source-commit comparison.
	if plan.Target.Digest == "" {
		return nil
	}
	return r.Remote.Run(fmt.Sprintf(
		"sudo install -d -m 0755 %s && printf '%%s\n' %s | sudo tee %s >/dev/null",
		shellQuote(appliedDigestDir), shellQuote(plan.Target.Digest), shellQuote(AppliedDigestPath)))
}
