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
	if !imageRefsMatch(current, plan.Target.Ref) {
		return fmt.Errorf("post-reboot image %q does not match target %q", current, plan.Target.Ref)
	}
	return nil
}
