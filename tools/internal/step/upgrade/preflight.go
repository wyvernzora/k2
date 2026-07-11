package upgrade

import "fmt"

// Preflight enforces refusable invariants surfaced by Resolve.
// Returns nil when the upgrade can safely proceed; otherwise returns
// the first refusal reason. Caller is expected to surface the error
// to the operator before any destructive phase runs.
func (r *Runner) Preflight(plan Plan) error {
	if plan.Target.Ref == "" {
		return fmt.Errorf("no target image resolved")
	}
	if !plan.QuorumOK {
		return fmt.Errorf("control-plane quorum check failed: %s", plan.QuorumImpact)
	}
	if plan.Current.Ref != "" && plan.Current.Ref == plan.Target.Ref {
		return fmt.Errorf("node already on %s; pass --source explicitly to force re-install", plan.Target.Ref)
	}
	if plan.StateTotalBytes < plan.Target.StateSizeBytes {
		return fmt.Errorf("COS_STATE is too small for target image: have %d MiB, target requires %d MiB",
			plan.StateTotalBytes>>20, plan.Target.StateSizeBytes>>20)
	}
	if plan.StateAvailableBytes < plan.RequiredFreeBytes {
		return fmt.Errorf("COS_STATE has insufficient free space: have %d MiB, need %d MiB (%d MiB image allowance + %d MiB safety margin)",
			plan.StateAvailableBytes>>20, plan.RequiredFreeBytes>>20,
			plan.Target.UpgradeSizeAllowanceBytes>>20, UpgradeSafetyMarginBytes>>20)
	}
	return nil
}
