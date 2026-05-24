// Package upgrade is the pure-Go core of the `k2-tools upgrade`
// subcommand — orchestrates an in-place Kairos image refresh of a
// single node. The CLI in cmd/k2-tools owns the UI (the Workflow,
// the Plan section, the Confirm prompt); this package owns the
// state machine + the I/O against the node + against the cluster.
//
// The split between this package and its call site mirrors how
// internal/flash/runner.go relates to cmd/k2-tools/flash.go:
//   - This package returns errors + populates a Plan struct. It
//     does NOT decide how to render anything.
//   - The caller wraps each phase in a Workflow step, deciding the
//     Section / Shell / Confirm shape.
//
// Why an explicit Plan struct rather than per-phase return values?
// The upgrade workflow's first user-visible artifact is the Plan
// section — the operator wants to see "what's about to happen"
// BEFORE they answer the Confirm prompt. Splitting Resolve into
// a single function that returns a full Plan lets the caller render
// every field in the Plan section without juggling N intermediate
// values. The per-phase functions then consume the same Plan as
// their source of truth; they don't re-derive anything.
package upgrade

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wyvernzora/k2/kairos/tools/internal/kubectl"
	"github.com/wyvernzora/k2/kairos/tools/internal/oci"
	"github.com/wyvernzora/k2/kairos/tools/internal/remote"
)

// DefaultRegistryRepo is the OCI repo `k2-tools upgrade` queries when
// auto-discovering "the newest image for this node's hardware/arch."
// Exported so a downstream consumer (or a fork) can swap it; the CLI
// reads this constant rather than hardcoding.
const DefaultRegistryRepo = "ghcr.io/wyvernzora/k2-kairos"

// MetadataPath is the absolute path on a Kairos node where
// k2-image-build bakes the target/arch/hardware metadata file. The
// upgrade subcommand reads it during Resolve to construct the
// tag-search prefix (and surface the current image in the Plan).
const MetadataPath = "/usr/share/k2/image-build/metadata.yaml"

// ActiveModePath is the Kairos runtime marker that names which COS_*
// partition is currently booted (active / passive / recovery). Verify
// reads it after the reboot to confirm we landed on active and not
// recovery.
const ActiveModePath = "/run/cos/active_mode"

// KairosReleasePath is /etc/kairos-release — the per-boot file that
// carries the OCI ref the running image was installed from
// (IMAGE_REPO + IMAGE_LABEL). We grep it to verify the post-reboot
// active image matches what kairos-agent upgrade was told to install.
const KairosReleasePath = "/etc/kairos-release"

// Defaults for the upgrade timeouts. Picked as the longest
// reasonable values for a CM4-on-eMMC node (slowest hardware we
// upgrade). Operators override per-invocation via CLI flags.
const (
	DefaultDrainTimeout  = 5 * time.Minute
	DefaultRebootTimeout = 10 * time.Minute
	DefaultVerifyTimeout = 3 * time.Minute
)

// Plan is the resolved set of facts an upgrade is about to act on.
// Produced by Resolve(); consumed by Confirm and every subsequent
// phase. Every field is operator-visible — render the whole struct
// in the Plan section.
type Plan struct {
	ClusterName    string
	Host           string
	SSHUser        string
	SSHPort        int
	NodeName       string
	IsControlPlane bool

	// Current is the image the node is presently running. We read
	// the ref from /etc/kairos-release IMAGE field. Digest may be
	// empty if /etc/kairos-release doesn't carry the digest form;
	// this is informational, not load-bearing.
	Current ImageRef

	// Target is the image the upgrade will install. Always populated;
	// when the operator passes --source, it equals that ref; when
	// auto-discovered, it's the newest registry tag matching the
	// node's target prefix.
	Target ImageRef

	// AutoDiscovered is true iff Target came from a registry query
	// rather than an operator-supplied --source. Used by the CLI to
	// decide whether to surface "auto-discovered from <repo>" in
	// the Plan.
	AutoDiscovered bool

	// QuorumImpact is a human-readable summary of the control-plane
	// quorum effect of taking this node down. Empty for worker
	// upgrades. Examples: "1 of 3 (2 remain — safe)",
	// "1 of 1 (refusing — pass --allow-quorum-loss to override)".
	QuorumImpact string

	// QuorumOK is true when an upgrade can safely proceed. False
	// indicates a CP upgrade that would drop quorum below the safe
	// threshold (1 remaining CP). Preflight refuses unless
	// AllowQuorumLoss was set.
	QuorumOK bool
}

// ImageRef is the small shape we carry around for "an OCI image we
// either know about or are about to install". Created is zero when
// the source of the ref is something that doesn't have a publish
// time (e.g. /etc/kairos-release on the node). Digest is best-effort:
// we record it when the registry tells us, empty otherwise.
type ImageRef struct {
	Ref     string
	Digest  string
	Created time.Time
}

// String renders ImageRef as the operator wants to see it in the
// Plan: the bare ref. Callers compose age separately.
func (i ImageRef) String() string {
	if i.Ref == "" {
		return "(unknown)"
	}
	return i.Ref
}

// Inputs is the per-invocation operator intent — the CLI flags
// translated into a struct. Pass into Resolve.
type Inputs struct {
	ClusterName     string
	Host            string
	SSHUser         string
	SSHPort         int
	Source          string // empty → auto-discover
	RegistryRepo    string // empty → DefaultRegistryRepo
	AllowQuorumLoss bool
}

// Runner ties together the three I/O surfaces an upgrade touches:
// the node (via SSH), the cluster (via local kubectl), and the
// registry (via crane). Construct once at the call site, pass to
// every phase. None of the fields can be nil at invocation time;
// the CLI is responsible for wiring them.
type Runner struct {
	Remote   *remote.Client
	Kubectl  *kubectl.Client
	Registry *oci.Discoverer

	// MetadataReader is the function used to read /usr/share/k2/.../metadata.yaml
	// off the node and decode it. Wired to the existing
	// readRemoteMetadata in main.go so we don't duplicate decode logic.
	// Returns the prefix-relevant fields we feed into the tag search.
	MetadataReader func(*remote.Client) (NodeImageMetadata, error)
}

// NodeImageMetadata is the slimmed shape the upgrade package needs
// from the on-node metadata.yaml. Mirrors render.ImageMetadata but
// kept distinct so this package doesn't import the render package
// (which carries the bootstrap-only types). The CLI adapter
// translates between the two.
type NodeImageMetadata struct {
	Target   string
	Arch     string
	Hardware string
}

// Resolve runs every read-only step needed to populate a Plan: SSH
// metadata read, current-image-ref read, registry discovery (when
// --source omitted), kubectl node lookup, control-plane quorum
// math. Returns a Plan ready to be rendered to the operator.
//
// Resolve is purely informational — no node state is modified, no
// cluster state is modified. The operator's Confirm answer gates
// every subsequent phase.
func (r *Runner) Resolve(ctx context.Context, in Inputs) (Plan, error) {
	if err := r.validate(); err != nil {
		return Plan{}, err
	}
	plan := Plan{
		ClusterName: in.ClusterName,
		Host:        in.Host,
		SSHUser:     in.SSHUser,
		SSHPort:     in.SSHPort,
	}

	// Match SSH host to a kubernetes Node via InternalIP. This pre-flight
	// also catches kubeconfig wiring + cluster-reachability failures
	// EARLY — before we SSH anywhere — so failure mode is "wrong cluster?
	// kubeconfig missing?" and not "spent 10s SSHing then choked on
	// kubectl".
	node, err := r.Kubectl.FindNodeByInternalIP(ctx, in.Host)
	if err != nil {
		return Plan{}, fmt.Errorf("locate node by InternalIP: %w", err)
	}
	plan.NodeName = node.Name
	plan.IsControlPlane = node.IsControlPlane()

	// Read on-node metadata over SSH.
	meta, err := r.MetadataReader(r.Remote)
	if err != nil {
		return Plan{}, fmt.Errorf("read node metadata: %w", err)
	}

	// Read /etc/kairos-release for the current image ref.
	current, err := readCurrentImage(r.Remote)
	if err != nil {
		// Not fatal — Resolve still works, just shows "(unknown)" in
		// the Plan. This happens on older Kairos releases that don't
		// carry IMAGE_REPO/IMAGE_LABEL in /etc/kairos-release.
		current = ""
	}
	plan.Current = ImageRef{Ref: current}

	// Resolve the target image.
	if in.Source != "" {
		// Explicit --source. Hit the registry just for the publish
		// date + digest so the Plan still has the "published N days
		// ago" line.
		img, err := r.Registry.InspectImage(ctx, in.Source)
		if err != nil {
			// We tolerate registry failure for the explicit-source
			// case: the operator already told us what to install,
			// the Plan just won't show the age.
			plan.Target = ImageRef{Ref: in.Source}
		} else {
			plan.Target = ImageRef{Ref: img.Ref, Digest: img.Digest, Created: img.Created}
		}
		plan.AutoDiscovered = false
	} else {
		repo := in.RegistryRepo
		if repo == "" {
			repo = DefaultRegistryRepo
		}
		prefix := tagPrefix(meta)
		img, err := r.Registry.DiscoverLatest(ctx, repo, prefix)
		if err != nil {
			return Plan{}, fmt.Errorf("auto-discover latest image: %w; pass --source <ref> explicitly", err)
		}
		plan.Target = ImageRef{Ref: img.Ref, Digest: img.Digest, Created: img.Created}
		plan.AutoDiscovered = true
	}

	// Control-plane quorum check. Worker nodes always pass.
	if plan.IsControlPlane {
		nodes, err := r.Kubectl.Nodes(ctx)
		if err != nil {
			return Plan{}, fmt.Errorf("list nodes for quorum check: %w", err)
		}
		plan.QuorumImpact, plan.QuorumOK = quorumImpact(nodes, plan.NodeName, in.AllowQuorumLoss)
	} else {
		plan.QuorumOK = true
	}

	return plan, nil
}

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
	return nil
}

// Cordon marks the kubernetes node unschedulable.
func (r *Runner) Cordon(ctx context.Context, plan Plan) error {
	return r.Kubectl.Cordon(ctx, plan.NodeName)
}

// Drain evicts pods off the node. Streams kubectl's eviction log
// through the configured Kubectl.Stdout (the caller wires this to a
// ui.Step's writer).
func (r *Runner) Drain(ctx context.Context, plan Plan, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultDrainTimeout
	}
	return r.Kubectl.Drain(ctx, plan.NodeName, kubectl.DrainOpts{
		Timeout:            timeout,
		IgnoreDaemonsets:   true,
		DeleteEmptyDirData: true,
		GracePeriodSeconds: -1,
	})
}

// UpgradeActive runs `sudo kairos-agent upgrade --source <ref>` on
// the node, which writes the new image to COS_ACTIVE. Does NOT
// reboot — Reboot is the next phase.
func (r *Runner) UpgradeActive(ctx context.Context, plan Plan) error {
	script := fmt.Sprintf("sudo kairos-agent upgrade --source %s", shellQuote(plan.Target.Ref))
	return r.Remote.Run(script)
}

// Reboot triggers a reboot and waits for SSH to come back up.
// `timeout` caps the wait; on expiry we return a wrapped timeout
// error and the caller decides whether the node is broken or just
// slow.
func (r *Runner) Reboot(ctx context.Context, plan Plan, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultRebootTimeout
	}
	if err := r.Remote.RunAllowDisconnect("sudo reboot"); err != nil {
		return fmt.Errorf("reboot command: %w", err)
	}
	// Give the node a moment to actually drop the SSH session + kill
	// sshd before we start probing. Otherwise our first probe finds
	// the OLD sshd still alive and we declare the reboot "instant".
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
	}
	if err := r.Remote.WaitForAuth(timeout); err != nil {
		return fmt.Errorf("wait for SSH after reboot: %w", err)
	}
	return nil
}

// VerifyActive confirms the post-reboot node is running the target
// image AND booted active (not recovery). Returns a clear error if
// either check fails — the caller leaves the node cordoned and
// surfaces the error.
func (r *Runner) VerifyActive(ctx context.Context, plan Plan) error {
	mode, err := r.Remote.Capture("cat " + shellQuote(ActiveModePath) + " 2>/dev/null || true")
	if err != nil {
		return fmt.Errorf("read %s: %w", ActiveModePath, err)
	}
	trimmed := strings.TrimSpace(string(mode))
	if trimmed != "" && trimmed != "active" {
		return fmt.Errorf("node booted into %q mode, not active; manual recovery required", trimmed)
	}
	current, err := readCurrentImage(r.Remote)
	if err != nil {
		return fmt.Errorf("read post-reboot image ref: %w", err)
	}
	if !imageRefsMatch(current, plan.Target.Ref) {
		return fmt.Errorf("post-reboot image %q does not match target %q", current, plan.Target.Ref)
	}
	return nil
}

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

// UpgradeRecovery syncs the COS_RECOVERY partition with the new
// image. Failure here is logged at the call site but does NOT block
// uncordoning — the node is functionally upgraded; recovery is a
// belt-and-suspenders concern that can be re-run later.
func (r *Runner) UpgradeRecovery(ctx context.Context, plan Plan) error {
	script := fmt.Sprintf("sudo kairos-agent upgrade --recovery --source %s",
		shellQuote(plan.Target.Ref))
	return r.Remote.Run(script)
}

// Uncordon marks the kubernetes node schedulable again.
func (r *Runner) Uncordon(ctx context.Context, plan Plan) error {
	return r.Kubectl.Uncordon(ctx, plan.NodeName)
}

// ----- helpers ----------------------------------------------------------

func (r *Runner) validate() error {
	if r.Remote == nil {
		return fmt.Errorf("upgrade.Runner: Remote is required")
	}
	if r.Kubectl == nil {
		return fmt.Errorf("upgrade.Runner: Kubectl is required")
	}
	if r.Registry == nil {
		return fmt.Errorf("upgrade.Runner: Registry is required")
	}
	if r.MetadataReader == nil {
		return fmt.Errorf("upgrade.Runner: MetadataReader is required")
	}
	return nil
}

// tagPrefix constructs the OCI tag-search prefix from the on-node
// metadata. The image-build pipeline names tags as
// `<target>-<k3sVersion>-rev<N>` (rev increments per content-hash
// change). We don't have k3sVersion in metadata.yaml today, so the
// prefix is just `<target>-`; the registry typically only carries
// one k3s version per target at a time, and any mismatch surfaces
// to the operator in the Plan as "Target image: <ref>" — they
// notice if the k3s suffix changed unexpectedly.
//
// If metadata.yaml ever grows a `k3sVersion` field, tighten this
// to include it.
func tagPrefix(meta NodeImageMetadata) string {
	return meta.Target + "-"
}

// readCurrentImage SSH-reads /etc/kairos-release and extracts the
// IMAGE_REPO + IMAGE_LABEL composite ref. Returns "" + error if the
// file is missing or doesn't carry IMAGE_REPO; the caller treats
// this as "unknown current image, just show Target."
func readCurrentImage(client *remote.Client) (string, error) {
	data, err := client.Capture("cat " + shellQuote(KairosReleasePath))
	if err != nil {
		return "", err
	}
	return parseImageRef(string(data)), nil
}

// parseImageRef extracts the OCI ref from /etc/kairos-release's
// IMAGE_REPO + IMAGE_LABEL pair. Format:
//
//	IMAGE_REPO=ghcr.io/wyvernzora/k2-kairos
//	IMAGE_LABEL=ubuntu-24.04-standard-arm64-rpi4cb-k3s-v1.36.0-k3s1-rev3
//
// Returns "" if either field is missing.
func parseImageRef(release string) string {
	var repo, label string
	for _, line := range strings.Split(release, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "IMAGE_REPO="); ok {
			repo = trimEnvValue(v)
		}
		if v, ok := strings.CutPrefix(line, "IMAGE_LABEL="); ok {
			label = trimEnvValue(v)
		}
	}
	if repo == "" || label == "" {
		return ""
	}
	return repo + ":" + label
}

func trimEnvValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		v = v[1 : len(v)-1]
	}
	return v
}

// imageRefsMatch compares two refs by string equality after
// trimming whitespace. We do NOT compare by digest here — if the
// node reports `repo:label` and the operator's target was the same
// `repo:label`, the upgrade was applied. A digest comparison would
// be stricter but kairos-agent doesn't always return a digested ref
// in /etc/kairos-release.
func imageRefsMatch(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

// quorumImpact summarizes the CP-quorum impact of taking
// `upgradingNode` down. Worker upgrades don't get here. Returns a
// human-readable line + a bool indicating whether the upgrade is
// safe to proceed. For etcd, "safe" means at least one OTHER CP is
// Ready (so quorum survives one taken-down CP). Single-CP clusters
// always refuse unless allowQuorumLoss is true (the operator's
// explicit consent to "yes I know this kills the cluster").
func quorumImpact(nodes []kubectl.Node, upgradingNode string, allowQuorumLoss bool) (string, bool) {
	var (
		totalCP    int
		readyCP    int
		otherReady int
	)
	for _, n := range nodes {
		if !n.IsControlPlane() {
			continue
		}
		totalCP++
		if !n.Ready() {
			continue
		}
		readyCP++
		if n.Name != upgradingNode {
			otherReady++
		}
	}
	if otherReady >= 1 {
		return fmt.Sprintf("1 of %d CP (%d remain Ready — safe)", totalCP, otherReady), true
	}
	msg := fmt.Sprintf("0 of %d other CP Ready (etcd quorum will be lost)", totalCP-1)
	if allowQuorumLoss {
		return msg + " — proceeding due to --allow-quorum-loss", true
	}
	return msg + " — refuse (pass --allow-quorum-loss to override)", false
}

// shellQuote is the same simple POSIX single-quote escape used by
// internal/remote. Duplicated here to avoid exposing the helper
// publicly from that package.
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}
