// Package upgrade is the pure-Go core of the `k2-tools upgrade`
// subcommand — orchestrates an in-place Kairos image refresh of a
// single node. The adapter in tools/internal/workflow owns the UI
// (the Workflow, the Plan section, the Confirm prompt); this package
// owns the state machine + the I/O against the node + against the cluster.
//
// The split between this package and its call site mirrors how
// tools/internal/step/flash/runner.go relates to tools/internal/workflow/image/flash.go:
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
	"strconv"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
	"github.com/wyvernzora/k2/tools/internal/client/oci"
	"github.com/wyvernzora/k2/tools/internal/client/remote"
)

// DefaultRegistryRepo is the OCI repo `k2-tools upgrade` queries when
// auto-discovering "the newest image for this node's hardware/arch."
// Exported so a downstream consumer (or a fork) can swap it; the CLI
// reads this constant rather than hardcoding.
const DefaultRegistryRepo = "ghcr.io/wyvernzora/k2-kairos"

// MetadataPath is the absolute path on a Kairos node where
// The Kairos image pipeline bakes the target/arch/hardware metadata file. The
// upgrade subcommand reads it during Resolve to construct the
// tag-search prefix (and surface the current image in the Plan).
const MetadataPath = "/usr/share/k2/image-build/metadata.yaml"

const (
	StateMountPath     = "/run/initramfs/cos-state"
	RecoveryDevicePath = "/dev/disk/by-label/COS_RECOVERY"
)

// ActiveModePath is the Kairos runtime marker created when the node boots
// from the active partition. Its contents are not meaningful.
const ActiveModePath = "/run/cos/active_mode"

// RecoveryModePath is the Kairos runtime marker created when the node boots
// from the recovery partition. Marker contents are not meaningful; boot mode
// is determined by which marker file exists.
const RecoveryModePath = "/run/cos/recovery_mode"

// Defaults for the upgrade timeouts. Picked as the longest
// reasonable values for a CM4-on-eMMC node (slowest hardware we
// upgrade). Operators override per-invocation via CLI flags.
const (
	DefaultDrainTimeout       = 5 * time.Minute
	DefaultRebootTimeout      = 10 * time.Minute
	DefaultVerifyTimeout      = 3 * time.Minute
	StateSafetyMarginBytes    = uint64(1 << 30)
	RecoverySafetyMarginBytes = uint64(200 << 20)
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

	// Current is the image the node is presently running, reconstructed
	// from K2's on-node metadata using the target repository.
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

	StateAvailableBytes       uint64
	RecoveryAvailableBytes    uint64
	RequiredStateFreeBytes    uint64
	RequiredRecoveryFreeBytes uint64
}

// ImageRef is the small shape we carry around for "an OCI image we
// either know about or are about to install". Created is zero when
// the source of the ref is something that doesn't have a publish
// time (e.g. metadata.yaml on the node). Digest is best-effort:
// we record it when the registry tells us, empty otherwise.
type ImageRef struct {
	Ref                    string
	Digest                 string
	Created                time.Time
	UpgradeAllocationBytes uint64
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
	// Returns the canonical K2 image identity used by the Plan and verifier.
	MetadataReader func(*remote.Client) (NodeImageMetadata, error)
}

// NodeImageMetadata is the K2-owned image identity baked into metadata.yaml.
// It supports both the legacy Ubuntu 24.04 target layout and the current
// Ubuntu 26.04 layout so already-installed nodes remain identifiable.
type NodeImageMetadata struct {
	Target                  string
	Flavor                  string
	FlavorRelease           string
	Variant                 string
	Role                    string
	Arch                    string
	Hardware                string
	KubernetesDistro        string
	KubernetesVersion       string
	KairosVersion           string
	ImageRevision           string
	DiskStateSizeMiB        uint64
	UpgradeSizeAllowanceMiB uint64
	RootFSSizeMiB           uint64
}

// Resolve runs every read-only step needed to populate a Plan: SSH
// metadata read, registry discovery (when --source omitted), kubectl node
// lookup, control-plane quorum
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

	// Resolve the target image.
	if in.Source != "" {
		// Explicit --source. Hit the registry just for the publish
		// date + digest so the Plan still has the "published N days
		// ago" line.
		img, err := r.Registry.InspectImage(ctx, in.Source)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect explicit source image: %w", err)
		}
		plan.Target = imageRef(img)
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
		plan.Target = imageRef(img)
		plan.AutoDiscovered = true
	}

	repo := imageRepository(plan.Target.Ref)
	current := imageRefFromMetadata(repo, meta)
	if current == "" {
		return Plan{}, fmt.Errorf("node image metadata is incomplete; cannot reconstruct current image")
	}
	plan.Current = ImageRef{Ref: current}

	stateAvailable, recoveryAvailable, err := r.upgradeStorageAvailable()
	if err != nil {
		return Plan{}, fmt.Errorf("inspect upgrade storage available space: %w", err)
	}
	plan.StateAvailableBytes = stateAvailable
	plan.RecoveryAvailableBytes = recoveryAvailable
	plan.RequiredStateFreeBytes = plan.Target.UpgradeAllocationBytes + StateSafetyMarginBytes
	plan.RequiredRecoveryFreeBytes = plan.Target.UpgradeAllocationBytes + RecoverySafetyMarginBytes

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

func imageRef(img oci.Image) ImageRef {
	return ImageRef{
		Ref:                    img.Ref,
		Digest:                 img.Digest,
		Created:                img.Created,
		UpgradeAllocationBytes: img.UpgradeAllocationMiB << 20,
	}
}

func (r *Runner) upgradeStorageAvailable() (state uint64, recovery uint64, err error) {
	out, err := r.Remote.Capture(upgradeStorageProbeScript())
	if err != nil {
		return 0, 0, err
	}
	return parseUpgradeStorageAvailable(out)
}

func upgradeStorageProbeScript() string {
	return `set -eu
state_available=$(df -B1 --output=avail ` + shellQuote(StateMountPath) + ` | tail -n 1)
recovery_device=$(readlink -f ` + shellQuote(RecoveryDevicePath) + `)
recovery_mount=$(findmnt -rn -S "$recovery_device" -o TARGET | head -n 1 || true)
probe_mount=
cleanup() {
  if [ -n "$probe_mount" ]; then
    sudo umount "$probe_mount" >/dev/null 2>&1 || true
    rmdir "$probe_mount" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
if [ -z "$recovery_mount" ]; then
  probe_mount=$(mktemp -d /tmp/k2-recovery-probe.XXXXXX)
  sudo mount -o ro "$recovery_device" "$probe_mount"
  recovery_mount=$probe_mount
fi
recovery_available=$(df -B1 --output=avail "$recovery_mount" | tail -n 1)
printf '%s\n%s\n' "$state_available" "$recovery_available"
`
}

func parseUpgradeStorageAvailable(out []byte) (state uint64, recovery uint64, err error) {
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected upgrade storage available-space output %q", strings.TrimSpace(string(out)))
	}
	state, err = strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse COS_STATE available bytes %q: %w", fields[0], err)
	}
	recovery, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse COS_RECOVERY available bytes %q: %w", fields[1], err)
	}
	return state, recovery, nil
}

func bootModeProbeScript(activePath, recoveryPath string) string {
	return fmt.Sprintf(
		"if test -e %s; then printf recovery; elif test -e %s; then printf active; else printf unknown; fi",
		shellQuote(recoveryPath), shellQuote(activePath),
	)
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

// tagPrefix constructs the stable target portion of the OCI tag while leaving
// the Kubernetes version and revision open for registry discovery.
func tagPrefix(meta NodeImageMetadata) string {
	segments := imageTagBase(meta)
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, "-") + "-"
}

func imageRefFromMetadata(repo string, meta NodeImageMetadata) string {
	segments := imageTagBase(meta)
	if repo == "" || len(segments) == 0 || meta.ImageRevision == "" {
		return ""
	}
	if meta.KubernetesDistro != "" {
		if meta.KubernetesVersion == "" {
			return ""
		}
		segments = append(segments, strings.ReplaceAll(meta.KubernetesVersion, "+", "-"))
	}
	segments = append(segments, meta.ImageRevision)
	return repo + ":" + strings.Join(segments, "-")
}

func imageTagBase(meta NodeImageMetadata) []string {
	flavor := meta.Flavor
	if meta.FlavorRelease != "" {
		flavor += "-" + meta.FlavorRelease
	}
	if meta.Variant != "" {
		flavor += "-" + meta.Variant
	}
	role := meta.Role
	if role == "" {
		role = meta.KubernetesDistro
	}
	if flavor == "" || meta.KairosVersion == "" || meta.Arch == "" || meta.Hardware == "" || role == "" {
		return nil
	}
	return []string{flavor, meta.KairosVersion, meta.Arch, meta.Hardware, role}
}

func imageRepository(ref string) string {
	lastSlash := strings.LastIndex(ref, "/")
	if at := strings.LastIndex(ref, "@"); at > lastSlash {
		return ref[:at]
	}
	if colon := strings.LastIndex(ref, ":"); colon > lastSlash {
		return ref[:colon]
	}
	return ""
}

func kairosUpgradeSource(ref string) string {
	return "oci:" + ref
}

// imageRefsMatch compares two refs by string equality after
// trimming whitespace. metadata.yaml records the tag components rather than
// the registry digest, so post-reboot verification intentionally compares tags.
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
