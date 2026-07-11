package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
	"github.com/wyvernzora/k2/tools/internal/client/oci"
	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/step/upgrade"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func upgradeRegistration() Registration {
	return Registration{Name: "upgrade", Help: "Upgrade a Kairos node's image in place.", Order: 50, Command: &upgradeCmd{}}
}

// upgradeCmd is `k2-tools upgrade` — in-place node image refresh.
// The flags mirror the conventions of the provision subcommands
// (cluster-name + SSH user/port/host + --yes) plus three
// upgrade-specific knobs (--source, drain/reboot timeouts,
// --allow-quorum-loss). See the plan file for the rationale.
type upgradeCmd struct {
	ClusterName string `name:"cluster-name" env:"K2_UPGRADE_CLUSTER_NAME" required:"" help:"Local cluster instance name (matches the dir under ~/.kube/k2/)."`
	Host        string `name:"host" env:"K2_UPGRADE_HOST" required:"" help:"InternalIP / SSH host of the node to upgrade."`
	SSHPort     int    `name:"ssh-port" env:"K2_UPGRADE_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser     string `name:"ssh-user" env:"K2_UPGRADE_SSH_USER" default:"kairos" help:"SSH user."`
	Identity    string `name:"identity" env:"K2_UPGRADE_IDENTITY" type:"path" help:"Unencrypted SSH private key for key auth (hardened nodes reject passwords; without this the client probes the default password first)."`

	Source       string `name:"source" env:"K2_UPGRADE_SOURCE" help:"Target OCI ref (e.g. ghcr.io/wyvernzora/k2-kairos:tag). When omitted, auto-discover the newest published image matching this node's hardware/arch."`
	RegistryRepo string `name:"registry-repo" env:"K2_UPGRADE_REGISTRY_REPO" help:"Override the registry repository used for auto-discovery. Defaults to ghcr.io/wyvernzora/k2-kairos."`

	DrainTimeout    time.Duration `name:"drain-timeout" env:"K2_UPGRADE_DRAIN_TIMEOUT" default:"5m" help:"Cap on kubectl drain duration."`
	RebootTimeout   time.Duration `name:"reboot-timeout" env:"K2_UPGRADE_REBOOT_TIMEOUT" default:"10m" help:"Cap on post-reboot SSH wait."`
	AllowQuorumLoss bool          `name:"allow-quorum-loss" env:"K2_UPGRADE_ALLOW_QUORUM_LOSS" help:"Permit upgrading a CP node when it's the only Ready CP. Single-CP test clusters need this; production CP HA never should."`

	Yes bool `name:"yes" short:"y" env:"K2_UPGRADE_YES" help:"Skip the Plan confirmation prompt. Required for non-TTY invocations."`
}

// Run wires the upgrade phases into a ui.Workflow with a Plan +
// Confirm prelude. Mirrors the shape of bootstrapCmd.Run so an
// operator reading both sees the same beats: pre-flight → render
// Plan → Confirm → execute → final Banner.
func (c *upgradeCmd) Run(rcx *Runtime) error {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	prevCancel := currentReporter().SetInterruptCancel(cancel)
	defer currentReporter().SetInterruptCancel(prevCancel)

	// ---- inputs + I/O surfaces ------------------------------------------

	kubeconfigPath, err := kubeconfigPathFor(c.ClusterName)
	if err != nil {
		return err
	}
	kc := kubectl.New(kubeconfigPath)
	kc.Logger = logf
	if err := kc.Available(); err != nil {
		return fmt.Errorf("%w; install kubectl + ensure it's on PATH", err)
	}

	client := remote.Client{
		Host:         c.Host,
		Port:         c.SSHPort,
		User:         c.SSHUser,
		IdentityFile: c.Identity,
		Logger:       logf,
	}

	runner := &upgrade.Runner{
		Remote:         &client,
		Kubectl:        kc,
		Registry:       oci.New(),
		MetadataReader: readUpgradeMetadata,
	}

	in := upgrade.Inputs{
		ClusterName:     c.ClusterName,
		Host:            c.Host,
		SSHUser:         c.SSHUser,
		SSHPort:         c.SSHPort,
		Source:          c.Source,
		RegistryRepo:    c.RegistryRepo,
		AllowQuorumLoss: c.AllowQuorumLoss,
	}

	// ---- shared state captured by step closures -------------------------

	var (
		plan     upgrade.Plan
		started  time.Time
		recovErr error
	)

	// We resolve the Plan BEFORE the Workflow starts so a kubectl /
	// registry / SSH-metadata failure surfaces before any spinner —
	// the operator sees a clean error instead of "workflow step 1
	// (...) failed". Same posture as bootstrap's pre-Workflow
	// InspectExtraManifests.
	plan, err = runner.Resolve(parent, in)
	if err != nil {
		return err
	}
	if err := runner.Preflight(plan); err != nil {
		return err
	}

	wf := ui.NewWorkflow(currentReporter())

	// ---- plan + confirm -------------------------------------------------

	wf.Section("Plan")
	wf.KeyValues(upgradePlanFields(c, plan)...)
	wf.Confirm("Proceed with upgrade? [y/N]", "").Unless(c.Yes)

	// ---- upgrade --------------------------------------------------------

	wf.Section("Upgrade")

	wf.Shell("Cordon node", func(ctx context.Context, sh ui.Step) error {
		started = time.Now()
		defer client.SwapIO(sh)()
		kc.Stdout = sh
		kc.Stderr = sh
		defer func() { kc.Stdout = nil; kc.Stderr = nil }()
		return runner.Cordon(ctx, plan)
	})

	wf.Shell("Drain workloads", func(ctx context.Context, sh ui.Step) error {
		kc.Stdout = sh
		kc.Stderr = sh
		defer func() { kc.Stdout = nil; kc.Stderr = nil }()
		return runner.Drain(ctx, plan, c.DrainTimeout)
	})

	wf.Shell("kairos-agent upgrade (active partition)", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return runner.UpgradeActive(ctx, plan)
	})

	wf.Shell("Reboot + wait for SSH", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return runner.Reboot(ctx, plan, c.RebootTimeout)
	})

	wf.Shell("Verify active image", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return runner.VerifyActive(ctx, plan)
	})

	wf.Shell("Smoke-check node + pods", func(ctx context.Context, sh ui.Step) error {
		kc.Stdout = sh
		kc.Stderr = sh
		defer func() { kc.Stdout = nil; kc.Stderr = nil }()
		return runner.SmokeCheck(ctx, plan)
	})

	wf.Shell("kairos-agent upgrade (recovery partition)", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		// Capture but DON'T fail the workflow — recovery sync is
		// belt-and-suspenders; failure here leaves an upgraded but
		// not-fully-synced node, which is operator-runnable later.
		// The Banner surfaces the warning.
		recovErr = runner.UpgradeRecovery(ctx, plan)
		if recovErr != nil {
			sh.Warnf("recovery sync failed: %v (node is still in service)", recovErr)
			return nil
		}
		sh.Successf("recovery partition synced")
		return nil
	})

	wf.Shell("Uncordon node", func(ctx context.Context, sh ui.Step) error {
		kc.Stdout = sh
		kc.Stderr = sh
		defer func() { kc.Stdout = nil; kc.Stderr = nil }()
		return runner.Uncordon(ctx, plan)
	})

	wf.BannerFn(ui.BannerSuccess, func() []string {
		elapsed := time.Since(started).Truncate(time.Second)
		lines := []string{
			"Upgrade complete",
			fmt.Sprintf("%s now on %s (took %s)", plan.NodeName, plan.Target.Ref, elapsed),
		}
		if recovErr != nil {
			lines = append(lines,
				"NOTE: recovery sync failed — node is functional but rollback via reset would restore OLD image.",
				fmt.Sprintf("Retry: ssh %s@%s \"sudo kairos-agent upgrade --recovery --source %s\"",
					c.SSHUser, c.Host, plan.Target.Ref),
			)
		}
		return lines
	})

	return wf.Execute(parent)
}

// upgradePlanFields renders the operator-facing summary of every
// CLI arg / discovered fact the upgrade is about to act on. Keep
// this boring + factual — surprises here are failures to wire intent
// through, not stylistic choices.
func upgradePlanFields(c *upgradeCmd, plan upgrade.Plan) []ui.KV {
	pairs := []ui.KV{
		{Key: "Cluster", Value: plan.ClusterName},
		{Key: "Host", Value: fmt.Sprintf("%s@%s:%d", plan.SSHUser, plan.Host, plan.SSHPort)},
		{Key: "Node", Value: plan.NodeName},
		{Key: "Role", Value: nodeRoleLabel(plan.IsControlPlane)},
	}
	if plan.IsControlPlane {
		pairs = append(pairs, ui.KV{Key: "Quorum", Value: plan.QuorumImpact})
	}
	pairs = append(pairs,
		ui.KV{Key: "Current image", Value: plan.Current.String()},
		ui.KV{Key: "Target image", Value: targetWithAge(plan)},
		ui.KV{Key: "COS_STATE available", Value: humanBytesBinary(plan.StateAvailableBytes)},
		ui.KV{Key: "COS_RECOVERY available", Value: humanBytesBinary(plan.RecoveryAvailableBytes)},
		ui.KV{Key: "Required state free", Value: fmt.Sprintf("%s (%s allocation + %s margin)", humanBytesBinary(plan.RequiredStateFreeBytes), humanBytesBinary(plan.Target.UpgradeAllocationBytes), humanBytesBinary(upgrade.StateSafetyMarginBytes))},
		ui.KV{Key: "Required recovery free", Value: fmt.Sprintf("%s (%s allocation + %s margin)", humanBytesBinary(plan.RequiredRecoveryFreeBytes), humanBytesBinary(plan.Target.UpgradeAllocationBytes), humanBytesBinary(upgrade.RecoverySafetyMarginBytes))},
	)
	if plan.AutoDiscovered {
		repo := c.RegistryRepo
		if repo == "" {
			repo = upgrade.DefaultRegistryRepo
		}
		pairs = append(pairs, ui.KV{Key: "Discovered from", Value: repo})
	}
	pairs = append(pairs,
		ui.KV{Key: "Drain timeout", Value: c.DrainTimeout.String()},
		ui.KV{Key: "Reboot timeout", Value: c.RebootTimeout.String()},
		ui.KV{Key: "Sequence", Value: "cordon → drain → upgrade active → reboot → verify → upgrade recovery → uncordon"},
	)
	return pairs
}

func targetWithAge(plan upgrade.Plan) string {
	if plan.Target.Created.IsZero() {
		return plan.Target.Ref
	}
	return fmt.Sprintf("%s (published %s, %s)",
		plan.Target.Ref,
		plan.Target.Created.Format("2006-01-02"),
		humanAgo(time.Since(plan.Target.Created)),
	)
}

func humanBytesBinary(bytes uint64) string {
	const gib = uint64(1 << 30)
	return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gib))
}

// humanAgo renders a Duration as "3 days ago" / "5 hours ago" /
// "just now". Coarse on purpose — the Plan is for a human glance.
func humanAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	}
}

func nodeRoleLabel(isCP bool) string {
	if isCP {
		return "control-plane"
	}
	return "worker"
}
