package provision

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/render"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func provisionJoinNode(rcx *Runtime, role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) error {
	return runJoinProvision(context.Background(), rcx, role, flags, remoteFlags)
}

func runJoinProvision(parent context.Context, rcx *Runtime, role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) error {
	testTarget, err := applyProvisionTestVM(rcx.RepoRoot, flags.ClusterTarget, &flags.ClusterName, &flags.NodeName, &remoteFlags.Host, &remoteFlags.SSHPort, remoteFlags.TestVM)
	if err != nil {
		return err
	}
	if flags.NodeName == "" {
		return fmt.Errorf("missing node name; pass --node-name or --test-vm")
	}
	_ = testTarget // currently unused for join nodes; kept for future use

	client := remote.Client{
		Host:             remoteFlags.Host,
		Port:             remoteFlags.SSHPort,
		User:             remoteFlags.SSHUser,
		IdentityFile:     remoteFlags.Identity,
		InsecureHostKey:  remoteFlags.TestVM != "",
		NoPasswordPrompt: remoteFlags.noPasswordPrompt,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Logger:           logf,
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	prevCancel := currentReporter().SetInterruptCancel(cancel)
	defer currentReporter().SetInterruptCancel(prevCancel)

	var (
		metadata  render.ImageMetadata
		bundle    joinBundle
		localDir  string
		remoteDir string
	)

	wf := ui.NewWorkflow(currentReporter())

	wf.Section("Plan")
	wf.KeyValues(joinPlanFields(role, flags, remoteFlags)...)
	wf.Confirm("Proceed with provisioning? [y/N]", "").Unless(remoteFlags.Yes)

	wf.Section(fmt.Sprintf("Provision %s", role))
	wf.Shell("Read remote image metadata", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		var err error
		metadata, err = readRemoteMetadata(&client)
		if err != nil {
			return fmt.Errorf("%w; rebuild the image with baked metadata support", err)
		}
		return nil
	})

	wf.Section("Render bundle")
	wf.Task("Render join bundle", func(ctx context.Context) error {
		var err error
		bundle, err = buildJoinBundle(rcx.RepoRoot, role, flags, metadata)
		return err
	})
	wf.Task("Stage bundle locally", func(ctx context.Context) error {
		var err error
		localDir, err = os.MkdirTemp("", "k2-tools-"+string(role)+"-*")
		if err != nil {
			return err
		}
		wf.Defer(func() { _ = os.RemoveAll(localDir) })
		return writeJoinBundle(localDir, role, bundle)
	})
	wf.KeyValuesFn(func() []ui.KV {
		return []ui.KV{{Key: "Staging dir", Value: localDir}}
	})

	wf.Section("Upload + install")
	wf.Shell(fmt.Sprintf("Upload %s bundle to remote", role), func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		var err error
		remoteDir, err = client.UploadDir(localDir)
		if err != nil {
			return err
		}
		sh.Successf("uploaded to %s", remoteDir)
		return nil
	})
	wf.Shell("Run remote install script", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		if err := client.RunAllowDisconnect(joinInstallScript(remoteDir, flags.NodeName, role, remoteFlags.NoReboot)); err != nil {
			return err
		}
		if remoteFlags.NoReboot {
			sh.Successf("install complete; reboot skipped")
		} else {
			sh.Successf("install complete; node rebooting")
		}
		return nil
	})

	postInstall := !remoteFlags.NoReboot
	wf.Section("Post-reboot").Unless(!postInstall)
	wf.Shell("Wait for SSH after reboot", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		if err := sleepCtx(ctx, 10*time.Second); err != nil {
			return err
		}
		if err := client.WaitForAuthCtx(ctx, 5*time.Minute); err != nil {
			return err
		}
		sh.Successf("%s node %s accepted SSH", role, flags.NodeName)
		return nil
	}).Unless(!postInstall)
	wf.Shell("Verify provisioning", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		// 10m, not 5m: a fresh cluster must pull Cilium/kube-vip images before
		// the API VIP answers, and the joining agent blocks on that VIP.
		return verifyRemoteProvisioning(ctx, &client, string(role)+" node "+flags.NodeName, joinVerificationScript(flags.NodeName, role), 10*time.Minute)
	}).Unless(!postInstall)
	wf.Shell("Harden default access", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return hardenRemoteDefaultAccess(&client)
	}).Unless(!postInstall)
	wf.Shell("Mark worker as Longhorn storage node", func(ctx context.Context, sh ui.Step) error {
		if err := markLonghornStorageWorker(ctx, clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget), flags.NodeName, sh); err != nil {
			return err
		}
		sh.Successf("marked %s for Longhorn replica storage (%s)", flags.NodeName, longhornStorageNodeTag)
		return nil
	}).Unless(!postInstall || role != nodeRoleWorker)

	wf.BannerFn(ui.BannerSuccess, func() []string {
		if remoteFlags.NoReboot {
			return []string{
				fmt.Sprintf("%s install complete", role),
				"Reboot skipped — k3s left stopped on the node.",
				fmt.Sprintf("Bundle staged at %s", remoteDir),
			}
		}
		return []string{
			fmt.Sprintf("%s provisioning complete", role),
			fmt.Sprintf("Node %s joined cluster %s", flags.NodeName, clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget)),
		}
	})

	return wf.Execute(ctx)
}
