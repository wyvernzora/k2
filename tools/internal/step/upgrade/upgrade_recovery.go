package upgrade

import (
	"context"
	"fmt"
)

// UpgradeRecovery syncs the COS_RECOVERY partition with the new
// image. Failure here is logged at the call site but does NOT block
// uncordoning — the node is functionally upgraded; recovery is a
// belt-and-suspenders concern that can be re-run later.
func (r *Runner) UpgradeRecovery(ctx context.Context, plan Plan) error {
	script := fmt.Sprintf("sudo kairos-agent upgrade --recovery --source %s",
		shellQuote(kairosUpgradeSource(plan.Target.Ref)))
	return r.Remote.Run(script)
}
