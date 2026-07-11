package upgrade

import (
	"context"
	"fmt"
)

// UpgradeActive runs `sudo kairos-agent upgrade --source <ref>` on
// the node, which writes the new image to COS_ACTIVE. Does NOT
// reboot — Reboot is the next phase.
func (r *Runner) UpgradeActive(ctx context.Context, plan Plan) error {
	script := fmt.Sprintf("sudo kairos-agent upgrade --source %s", shellQuote(kairosUpgradeSource(plan.Target.Ref)))
	return r.Remote.Run(script)
}
