package upgrade

import (
	"context"
	"fmt"
	"strings"
)

// VerifyActive confirms the post-reboot node is running the target
// image AND booted active (not recovery). Returns a clear error if
// either check fails — the caller leaves the node cordoned and
// surfaces the error.
func (r *Runner) VerifyActive(ctx context.Context, plan Plan) error {
	mode, err := r.Remote.Capture(bootModeProbeScript(ActiveModePath, RecoveryModePath))
	if err != nil {
		return fmt.Errorf("read Kairos boot mode markers: %w", err)
	}
	trimmed := strings.TrimSpace(string(mode))
	if trimmed != "active" {
		return fmt.Errorf("node booted into %q mode, not active; manual recovery required", trimmed)
	}
	meta, err := r.MetadataReader(r.Remote)
	if err != nil {
		return fmt.Errorf("read post-reboot image metadata: %w", err)
	}
	current := imageRefFromMetadata(imageRepository(plan.Target.Ref), meta)
	if current == "" {
		return fmt.Errorf("post-reboot image metadata is incomplete")
	}
	// Two separate questions, and the slot tag can only answer the first.
	//
	// Same slot? An upgrade must never move a node between slots, so the
	// reconstructed tag has to be the one the node already occupied — this is
	// what catches a --source aimed at another role's or arch's image. It says
	// nothing about which build booted, because the tag carries no version.
	if !imageRefsMatch(current, plan.Current.Ref) {
		return fmt.Errorf("post-reboot image %q does not occupy the node's slot %q", current, plan.Current.Ref)
	}
	// Same build? Only the source commit baked into the booted image answers
	// that, and it must be answered here: the next step records the target
	// digest as this node's identity. When the target carries no revision label
	// (a non-K2 --source) there is nothing to compare and the build goes
	// unverified rather than falsely verified.
	if plan.TargetSourceCommit != "" && !imageRefsMatch(meta.SourceCommit, plan.TargetSourceCommit) {
		return fmt.Errorf("post-reboot source commit %q does not match target %q; the node did not boot the target build",
			strings.TrimSpace(meta.SourceCommit), plan.TargetSourceCommit)
	}
	return nil
}
