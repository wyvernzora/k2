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
	if plan.StateAvailableBytes < plan.RequiredStateFreeBytes {
		return fmt.Errorf("COS_STATE has insufficient free space: have %d MiB, need %d MiB (%d MiB transition image + %d MiB safety margin)",
			plan.StateAvailableBytes>>20, plan.RequiredStateFreeBytes>>20,
			plan.Target.UpgradeAllocationBytes>>20, StateSafetyMarginBytes>>20)
	}
	if plan.RecoveryAvailableBytes < plan.RequiredRecoveryFreeBytes {
		return fmt.Errorf("COS_RECOVERY has insufficient free space: have %d MiB, need %d MiB (%d MiB transition image + %d MiB safety margin)",
			plan.RecoveryAvailableBytes>>20, plan.RequiredRecoveryFreeBytes>>20,
			plan.Target.UpgradeAllocationBytes>>20, RecoverySafetyMarginBytes>>20)
	}
	return nil
}
