package upgrade

import (
	"context"
	"fmt"
)

// UpgradeActive runs `sudo kairos-agent upgrade --source <ref>` on
// the node, which writes the new image to COS_ACTIVE. Does NOT
// reboot — Reboot is the next phase.
func (r *Runner) UpgradeActive(ctx context.Context, plan Plan) error {
	return r.Remote.Run(upgradeActiveScript(plan))
}

// upgradeActiveScript clears the applied-digest marker BEFORE writing the new
// image. From that instant until RecordAppliedDigest the marker can only lie:
// whichever image boots, the recorded digest names the previous one. Absence is
// the honest state — identity then falls back to the source commit baked into
// the booted image, which always describes what is actually running.
//
// This is also what makes a re-run resumable. ActiveImageMatchesTarget prefers
// the digest and does not fall through to the source commit when one is
// present, so a stale marker left across a failed `Verify active image` would
// report "not on target" for a node already running the target build and walk
// the operator through a second cordon → drain → upgrade → reboot (and, on a
// degraded control plane, hit Preflight's quorum refusal instead).
func upgradeActiveScript(plan Plan) string {
	return fmt.Sprintf("sudo rm -f %s && sudo kairos-agent upgrade --source %s",
		shellQuote(AppliedDigestPath), shellQuote(kairosUpgradeSource(plan.Target)))
}

// RecordAppliedDigest writes the marker that says "this is the image the node
// is running". The image cannot bake its own digest, so this marker on
// COS_PERSISTENT is how the next run — and the node's own k2-image-check
// timer — answers "is the tag still on the hash this node runs?" exactly
// rather than falling back to the coarser source-commit comparison.
//
// It MUST run only after VerifyActive has confirmed the node actually booted
// the target image: writing it next to `kairos-agent upgrade` would leave a
// node that failed to boot (or rolled back to the passive image) permanently
// claiming a digest it does not run, and nothing ever rewrites it. Its pair is
// the removal in upgradeActiveScript — together they leave the marker absent,
// never wrong, for the whole window in which it could be wrong. When the target
// digest is unknown the marker is removed here too, so identity falls back to
// the booted image's source commit rather than to the previous image's digest.
func (r *Runner) RecordAppliedDigest(ctx context.Context, plan Plan) error {
	if plan.Target.Digest == "" {
		return r.Remote.Run(fmt.Sprintf("sudo rm -f %s", shellQuote(AppliedDigestPath)))
	}
	return r.Remote.Run(fmt.Sprintf(
		"sudo install -d -m 0755 %s && printf '%%s\n' %s | sudo tee %s >/dev/null",
		shellQuote(appliedDigestDir), shellQuote(plan.Target.Digest), shellQuote(AppliedDigestPath)))
}
