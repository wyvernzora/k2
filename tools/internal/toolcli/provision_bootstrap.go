package toolcli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/manifests"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/remote"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/render"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

// bootstrapState carries the values that flow between Workflow steps.
// It exists so step closures can be hoisted out of Run into methods
// on bootstrapCmd.
type bootstrapState struct {
	client     *remote.Client
	testTarget testVMProvisionTarget
	extraObjs  []manifests.ExtraManifestObject
	metadata   render.ImageMetadata
	bundle     bundle
	localDir   string
	remoteDir  string
}

func (c *bootstrapCmd) Run(rcx *runContext) error {
	return runBootstrapProvision(context.Background(), rcx, c)
}

func runBootstrapProvision(parent context.Context, rcx *runContext, c *bootstrapCmd) error {
	testTarget, err := c.prepareTestVM(rcx)
	if err != nil {
		return err
	}
	extraObjs, err := manifests.InspectExtraManifests(c.ExtraManifests)
	if err != nil {
		return fmt.Errorf("inspect extra manifests for plan: %w", err)
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	prevCancel := reporter.SetInterruptCancel(cancel)
	defer reporter.SetInterruptCancel(prevCancel)

	state := &bootstrapState{
		client: &remote.Client{
			Host:             c.Host,
			Port:             c.SSHPort,
			User:             c.SSHUser,
			IdentityFile:     c.Identity,
			InsecureHostKey:  c.TestVM != "",
			NoPasswordPrompt: c.noPasswordPrompt,
			Stdout:           os.Stdout,
			Stderr:           os.Stderr,
			Logger:           logf,
		},
		testTarget: testTarget,
		extraObjs:  extraObjs,
	}

	wf := ui.NewWorkflow(reporter)
	c.buildBootstrapWorkflow(wf, rcx, state)
	return wf.Execute(ctx)
}

func (c *bootstrapCmd) buildBootstrapWorkflow(wf *ui.Workflow, rcx *runContext, s *bootstrapState) {
	postInstall := !c.NoReboot

	wf.Section("Plan")
	wf.KeyValuesFn(func() []ui.KV { return c.planKeyValues(s) })
	wf.Table(extraManifestHeaders, extraManifestRows(s.extraObjs)).
		Unless(len(s.extraObjs) == 0)
	wf.Confirm("Proceed with provisioning? [y/N]", "").Unless(c.Yes)

	wf.Section("Provision bootstrap")
	wf.Shell("Read remote image metadata", c.stepReadMetadata(s))
	wf.Shell("Detect bootstrap API host", c.stepDetectAPIHost(s)).
		When(func() bool { return c.BootstrapAPIHost == "" })

	wf.Section("Render bundle")
	wf.Task("Render bundle", c.stepRenderBundle(rcx, s))
	wf.Task("Stage bundle locally", c.stepStageBundleLocally(wf, s))
	wf.KeyValuesFn(func() []ui.KV {
		return []ui.KV{{Key: "Staging dir", Value: s.localDir}}
	})

	wf.Section("Upload + install")
	wf.Shell("Upload bootstrap bundle to remote", c.stepUploadBundle(s))
	wf.Shell("Run remote install script", c.stepRunInstall(s))

	wf.Section("Post-install").Unless(!postInstall)
	wf.Shell("Harvest bootstrap credentials", c.stepHarvest(rcx, s)).
		Unless(!postInstall)
	wf.Shell("Apply root Argo CD app", c.stepApplyRootArgoApp(s)).
		Unless(!postInstall)
	wf.Shell("Patch remote kube-vip", c.stepPatchKubeVIP(s)).
		Unless(!postInstall || s.testTarget.KubeVIP == "")
	wf.Shell("Verify provisioning", c.stepVerify(s)).
		Unless(!postInstall)
	wf.Shell("Harden default access", c.stepHarden(s)).
		Unless(!postInstall)

	wf.BannerFn(ui.BannerSuccess, func() []string { return c.bootstrapBanner(s) })
}

func (c *bootstrapCmd) planKeyValues(s *bootstrapState) []ui.KV {
	fields := bootstrapPlanFields(c, s.testTarget)
	if len(s.extraObjs) > 0 {
		fields = append(fields, ui.KV{Key: "Extra manifests", Value: fmt.Sprintf("%d object(s)", len(s.extraObjs))})
	} else if len(c.ExtraManifests) > 0 {
		fields = append(fields, ui.KV{Key: "Extra manifests", Value: "(no parseable objects found in supplied paths)"})
	}
	return fields
}

func (c *bootstrapCmd) stepReadMetadata(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		s.metadata, err = readRemoteMetadata(s.client)
		if err != nil {
			return fmt.Errorf("%w; rebuild the image with baked metadata support", err)
		}
		return nil
	}
}

func (c *bootstrapCmd) stepDetectAPIHost(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		c.BootstrapAPIHost, err = detectBootstrapAPIHost(s.client)
		if err != nil {
			return err
		}
		sh.Successf("API host %s", c.BootstrapAPIHost)
		return nil
	}
}

func (c *bootstrapCmd) stepRenderBundle(rcx *runContext, s *bootstrapState) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		s.bundle, err = buildBundle(rcx.repoRoot, c.commonBootstrapFlags, s.metadata)
		return err
	}
}

func (c *bootstrapCmd) stepStageBundleLocally(wf *ui.Workflow, s *bootstrapState) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		s.localDir, err = os.MkdirTemp("", "k2-tools-bootstrap-*")
		if err != nil {
			return err
		}
		wf.Defer(func() { _ = os.RemoveAll(s.localDir) })
		return writeBundle(s.localDir, s.bundle)
	}
}

func (c *bootstrapCmd) stepUploadBundle(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		s.remoteDir, err = s.client.UploadDir(s.localDir)
		if err != nil {
			return err
		}
		sh.Successf("uploaded to %s", s.remoteDir)
		return nil
	}
}

func (c *bootstrapCmd) stepRunInstall(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		if err := s.client.RunAllowDisconnect(installScript(s.remoteDir, c.NodeName, c.NoReboot)); err != nil {
			return err
		}
		if c.NoReboot {
			sh.Successf("install complete; reboot skipped")
		} else {
			sh.Successf("install complete; node rebooting")
		}
		return nil
	}
}

func (c *bootstrapCmd) stepHarvest(rcx *runContext, s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		cfg, err := clusterconfig.Load(rcx.repoRoot, c.ClusterTarget)
		if err != nil {
			return err
		}
		if s.testTarget.KubeVIP != "" {
			applyTestKubeVIP(&cfg, s.testTarget.KubeVIP)
		}
		clusterName := c.ClusterName
		if clusterName == "" {
			clusterName = c.ClusterTarget
		}
		return harvestBootstrapCredentials(ctx, s.client, cfg, clusterName)
	}
}

func (c *bootstrapCmd) stepPatchKubeVIP(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return patchRemoteKubeVIP(ctx, s.client, s.testTarget.KubeVIP, 3*time.Minute)
	}
}

func (c *bootstrapCmd) stepApplyRootArgoApp(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return applyRootArgoApp(ctx, s.client, 5*time.Minute)
	}
}

func (c *bootstrapCmd) stepVerify(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return verifyRemoteProvisioning(ctx, s.client, "bootstrap node "+c.NodeName, bootstrapVerificationScript(c.NodeName), 5*time.Minute)
	}
}

func (c *bootstrapCmd) stepHarden(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return hardenRemoteDefaultAccess(s.client)
	}
}

func (c *bootstrapCmd) bootstrapBanner(s *bootstrapState) []string {
	if c.NoReboot {
		return []string{
			"Bootstrap install complete",
			"Reboot skipped — k3s left stopped on the node.",
			fmt.Sprintf("Bundle staged at %s", s.remoteDir),
		}
	}
	return []string{
		"Bootstrap provisioning complete",
		fmt.Sprintf("Node %s joined cluster %s", c.NodeName, clusterNameOrFallback(c.ClusterName, c.ClusterTarget)),
	}
}

func (c *bootstrapCmd) prepareTestVM(ctx *runContext) (testVMProvisionTarget, error) {
	testTarget, err := applyProvisionTestVM(ctx.repoRoot, c.ClusterTarget, &c.ClusterName, &c.NodeName, &c.Host, &c.SSHPort, c.TestVM)
	if err != nil {
		return testVMProvisionTarget{}, err
	}
	if c.NodeName == "" {
		return testVMProvisionTarget{}, fmt.Errorf("missing node name; pass --node-name or --test-vm")
	}
	if !testTarget.Enabled {
		return testTarget, nil
	}
	if testTarget.GuestIP == "" || testTarget.KubeVIP == "" {
		return testVMProvisionTarget{}, fmt.Errorf("bootstrap --test-vm requires a guest IPv4 address from qemu guest agent")
	}
	c.commonBootstrapFlags.testKubeVIP = testTarget.KubeVIP
	if c.BootstrapAPIHost == "" {
		c.BootstrapAPIHost = testTarget.GuestIP
	}
	logf("using test VM %s: ssh %s:%d, cluster %s, bootstrap VIP %s", c.TestVM, c.Host, c.SSHPort, c.ClusterName, testTarget.KubeVIP)
	return testTarget, nil
}
