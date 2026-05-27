package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/kairos/tools/internal/clusterconfig"
	"github.com/wyvernzora/k2/kairos/tools/internal/keys"
	"github.com/wyvernzora/k2/kairos/tools/internal/kubeconfig"
	"github.com/wyvernzora/k2/kairos/tools/internal/kubectl"
	"github.com/wyvernzora/k2/kairos/tools/internal/manifests"
	"github.com/wyvernzora/k2/kairos/tools/internal/remote"
	"github.com/wyvernzora/k2/kairos/tools/internal/render"
	"github.com/wyvernzora/k2/kairos/tools/internal/ui"
	testvm "github.com/wyvernzora/k2/kairos/tools/internal/vm"
	"github.com/wyvernzora/k2/kairos/tools/internal/workspace"
)

type cli struct {
	RepoRoot string `name:"repo-root" env:"K2_TOOLS_REPO_ROOT" help:"Repository root. Defaults to auto-detection." type:"path"`
	Plain    bool   `name:"plain" env:"K2_TOOLS_PLAIN" help:"Use plain log output without grouped status markers."`

	Provision provisionCmd `cmd:"" help:"Provision Kairos-backed K3s nodes."`
	VM        vmCmd        `cmd:"" help:"Manage local test VMs."`
	Flash     flashCmd     `cmd:"" help:"Flash Kairos images to hardware."`
	Upgrade   upgradeCmd   `cmd:"" help:"Upgrade a Kairos node's image in place."`
}

type provisionCmd struct {
	Bootstrap bootstrapCmd `cmd:"" help:"Provision the first K3s server over SSH."`
	Server    serverCmd    `cmd:"" help:"Provision an additional K3s server over SSH."`
	Worker    workerCmd    `cmd:"" help:"Provision a K3s worker over SSH."`
	Render    renderCmd    `cmd:"" help:"Render provisioning files locally."`
}

type renderCmd struct {
	Bootstrap renderBootstrapCmd `cmd:"" help:"Render bootstrap provisioning files."`
}

type commonBootstrapFlags struct {
	ClusterTarget    string   `name:"cluster-target" env:"K2_PROVISION_CLUSTER_TARGET" required:"" help:"Cluster config/deploy target, such as v3."`
	ClusterName      string   `name:"cluster-name" env:"K2_PROVISION_CLUSTER_NAME" help:"Local cluster instance name. Defaults to cluster-target."`
	NodeName         string   `name:"node-name" env:"K2_PROVISION_NODE_NAME" help:"Kubernetes node name. Defaults to --test-vm when provided."`
	OperatorKey      []string `name:"operator-key" env:"K2_PROVISION_OPERATOR_KEY" help:"Literal ssh-ed25519 operator public key. Repeatable."`
	OperatorFiles    []string `name:"operator-key-file" env:"K2_PROVISION_OPERATOR_KEY_FILE" help:"File containing literal operator public keys. Repeatable." type:"path"`
	Label            []string `name:"label" env:"K2_PROVISION_LABEL" help:"Additional K3s node-label value. Repeatable."`
	Taint            []string `name:"taint" env:"K2_PROVISION_TAINT" help:"Additional K3s node-taint value. Repeatable."`
	BootstrapAPIHost string   `name:"bootstrap-api-host" env:"K2_PROVISION_BOOTSTRAP_API_HOST" help:"Kubernetes API host for bootstrap-only Cilium manifests. Bootstrap provisioning auto-detects the node IP when omitted."`

	ExtraManifests []string `name:"extra-manifests" env:"K2_PROVISION_EXTRA_MANIFESTS" help:"Extra bootstrap manifest path or glob to append verbatim. Repeatable."`

	testKubeVIP string
}

type bootstrapCmd struct {
	commonBootstrapFlags

	TestVM   string `name:"test-vm" env:"K2_PROVISION_TEST_VM" help:"Provision the local test VM id, defaulting host and cluster-name for VM swarm tests."`
	Host     string `name:"host" env:"K2_PROVISION_HOST" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" env:"K2_PROVISION_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" env:"K2_PROVISION_SSH_USER" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" env:"K2_PROVISION_NO_REBOOT" help:"Install files and enable k3s, but do not reboot."`
	Yes      bool   `name:"yes" env:"K2_PROVISION_YES" help:"Skip the plan confirmation prompt. Required for non-TTY invocations."`
}

type commonJoinFlags struct {
	ClusterTarget string   `name:"cluster-target" env:"K2_PROVISION_CLUSTER_TARGET" required:"" help:"Cluster config/deploy target, such as v3."`
	ClusterName   string   `name:"cluster-name" env:"K2_PROVISION_CLUSTER_NAME" help:"Local cluster instance name. Defaults to cluster-target."`
	NodeName      string   `name:"node-name" env:"K2_PROVISION_NODE_NAME" help:"Kubernetes node name. Defaults to --test-vm when provided."`
	OperatorKey   []string `name:"operator-key" env:"K2_PROVISION_OPERATOR_KEY" help:"Literal ssh-ed25519 operator public key. Repeatable."`
	OperatorFiles []string `name:"operator-key-file" env:"K2_PROVISION_OPERATOR_KEY_FILE" help:"File containing literal operator public keys. Repeatable." type:"path"`
	Label         []string `name:"label" env:"K2_PROVISION_LABEL" help:"Additional K3s node-label value. Repeatable."`
	Taint         []string `name:"taint" env:"K2_PROVISION_TAINT" help:"Additional K3s node-taint value. Repeatable."`
	ServerURL     string   `name:"server-url" env:"K2_PROVISION_SERVER_URL" help:"K3s API URL for joining. Defaults to ~/.kube/k2/<cluster-name>/server-url, then the API VIP from clusters/<target>.yaml."`
}

type commonRemoteFlags struct {
	TestVM   string `name:"test-vm" env:"K2_PROVISION_TEST_VM" help:"Provision the local test VM id, defaulting host and cluster-name for VM swarm tests."`
	Host     string `name:"host" env:"K2_PROVISION_HOST" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" env:"K2_PROVISION_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" env:"K2_PROVISION_SSH_USER" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" env:"K2_PROVISION_NO_REBOOT" help:"Install files and enable k3s, but do not reboot."`
	Yes      bool   `name:"yes" env:"K2_PROVISION_YES" help:"Skip the plan confirmation prompt. Required for non-TTY invocations."`
}

type serverCmd struct {
	commonJoinFlags
	commonRemoteFlags
}

type workerCmd struct {
	commonJoinFlags
	commonRemoteFlags
}

type renderBootstrapCmd struct {
	commonBootstrapFlags

	OutputDir string `name:"output-dir" env:"K2_PROVISION_OUTPUT_DIR" required:"" help:"Directory to write rendered files into." type:"path"`
}

type runContext struct {
	repoRoot string
}

type bundle struct {
	ClusterConfig   []byte
	BootstrapConfig []byte
	Activation      []byte
	AuthorizedKeys  []byte
	Manifests       []byte
	RootArgoApp     []byte
}

type joinBundle struct {
	ClusterConfig  []byte
	JoinConfig     []byte
	Activation     []byte
	AuthorizedKeys []byte
}

type nodeRole string

type testVMProvisionTarget struct {
	Enabled bool
	GuestIP string
	KubeVIP string
}

const (
	nodeRoleServer nodeRole = "server"
	nodeRoleWorker nodeRole = "worker"

	longhornStorageNodeLabel          = "node.longhorn.io/create-default-disk=true"
	longhornStorageNodeTag            = "k2-storage"
	longhornStorageNodeTagsAnnotation = `node.longhorn.io/default-node-tags=["k2-storage"]`
	longhornNodeLabelPrefix           = "node.longhorn.io/"

	remoteRootArgoAppManifestPath = "/var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml"
)

var reporter = ui.New(os.Stderr, false)

func main() {
	if err := run(os.Args[1:]); err != nil {
		reporter.Errorf("%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var app cli
	parser, err := kong.New(&app, kong.Name("k2-tools"), kong.UsageOnError())
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	reporter = ui.New(os.Stderr, app.Plain)
	repoRoot, err := workspace.FindRepoRoot(app.RepoRoot)
	if err != nil {
		return err
	}
	return ctx.Run(&runContext{repoRoot: repoRoot})
}

func (c *renderBootstrapCmd) Run(ctx *runContext) error {
	logf("render bootstrap bundle for target %s", c.ClusterTarget)
	bundle, err := buildBundle(ctx.repoRoot, c.commonBootstrapFlags, render.ImageMetadata{})
	if err != nil {
		return err
	}
	if err := writeBundle(c.OutputDir, bundle); err != nil {
		return err
	}
	successf("wrote bootstrap bundle to %s", c.OutputDir)
	return nil
}

// bootstrapState carries the values that flow between Workflow steps.
// It exists so step closures can be hoisted out of Run into methods
// on bootstrapCmd — moving each closure's internal control flow OUT
// of Run drops Run's cyclomatic complexity below the cyclop ceiling
// (15). The state stays a struct rather than a bag of locals so a
// Shell phase added later can just touch new fields without
// re-plumbing every closure.
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
	testTarget, err := c.prepareTestVM(rcx)
	if err != nil {
		return err
	}
	// Inspect extra-manifest objects BEFORE the workflow starts so we
	// can surface them in the Plan and abort on a parse error without
	// the operator first sitting through the SSH-metadata probe.
	extraObjs, err := manifests.InspectExtraManifests(c.ExtraManifests)
	if err != nil {
		return fmt.Errorf("inspect extra manifests for plan: %w", err)
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	reporter.SetInterruptCancel(cancel)
	defer reporter.SetInterruptCancel(nil)

	state := &bootstrapState{
		client: &remote.Client{
			Host:   c.Host,
			Port:   c.SSHPort,
			User:   c.SSHUser,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Logger: logf,
		},
		testTarget: testTarget,
		extraObjs:  extraObjs,
	}

	wf := ui.NewWorkflow(reporter)
	c.buildBootstrapWorkflow(wf, rcx, state)
	return wf.Execute(parent)
}

// buildBootstrapWorkflow registers every Step in declaration order.
// Each Shell closure is a thin adapter that calls into a named
// method on bootstrapCmd; the methods own the per-step logic that
// would otherwise inflate Run's cyclomatic complexity.
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
	wf.Step("render-bundle", c.stepRenderBundle(rcx, s))
	wf.Step("stage-bundle-locally", c.stepStageBundleLocally(wf, s))
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

// planKeyValues composes the Plan section's KV list. Lives here
// because its `if len(extraObjs) ... else if` would otherwise add
// branches to Run.
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
		return harvestBootstrapCredentials(s.client, cfg, clusterName)
	}
}

func (c *bootstrapCmd) stepPatchKubeVIP(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return patchRemoteKubeVIP(s.client, s.testTarget.KubeVIP, 3*time.Minute)
	}
}

func (c *bootstrapCmd) stepApplyRootArgoApp(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return applyRootArgoApp(s.client, 5*time.Minute)
	}
}

func (c *bootstrapCmd) stepVerify(s *bootstrapState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return verifyRemoteProvisioning(s.client, "bootstrap node "+c.NodeName, bootstrapVerificationScript(c.NodeName), 5*time.Minute)
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

func clusterNameOrFallback(name, target string) string {
	if name == "" {
		return target
	}
	return name
}

func nodeLabelsForRole(role nodeRole, labels []string) ([]string, error) {
	if role != nodeRoleWorker {
		if err := rejectLonghornNodeLabels(string(role), labels); err != nil {
			return nil, err
		}
		return labels, nil
	}
	out := append([]string{}, labels...)
	out = append(out, longhornStorageNodeLabel)
	return out, nil
}

func rejectLonghornNodeLabels(role string, labels []string) error {
	for _, label := range labels {
		key := strings.TrimSpace(label)
		if before, _, found := strings.Cut(key, "="); found {
			key = strings.TrimSpace(before)
		}
		if strings.HasPrefix(key, longhornNodeLabelPrefix) {
			return fmt.Errorf("%s provisioning does not allow user-supplied Longhorn node label %q; Longhorn replica storage is worker-only", role, key)
		}
	}
	return nil
}

// bootstrapPlanFields renders the operator-facing summary of every CLI
// arg / env var the bootstrap subcommand is about to act on. Used by
// the workflow Plan section right before the FLASH-equivalent yes/no
// confirmation. Keep this boring + factual — surprises here are
// failures to wire the operator's intent through, not stylistic
// choices.
func bootstrapPlanFields(c *bootstrapCmd, testTarget testVMProvisionTarget) []ui.KV {
	pairs := []ui.KV{
		{Key: "Cluster target", Value: c.ClusterTarget},
		{Key: "Cluster name", Value: clusterNameOrFallback(c.ClusterName, c.ClusterTarget)},
		{Key: "Node name", Value: c.NodeName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", c.SSHUser, c.Host, c.SSHPort)},
		{Key: "Operator keys", Value: keysSummary(c.OperatorKey, c.OperatorFiles)},
		{Key: "Labels", Value: joinOrNone(c.Label)},
		{Key: "Taints", Value: joinOrNone(c.Taint)},
		{Key: "Bootstrap API host", Value: hostOrAutoDetect(c.BootstrapAPIHost)},
		{Key: "Reboot after install", Value: yesNo(!c.NoReboot)},
	}
	if c.TestVM != "" {
		pairs = append(pairs,
			ui.KV{Key: "Test VM", Value: c.TestVM},
		)
		if testTarget.KubeVIP != "" {
			pairs = append(pairs, ui.KV{Key: "Test kube-VIP", Value: testTarget.KubeVIP})
		}
	}
	return pairs
}

// joinPlanFields = bootstrapPlanFields' shape for server/worker
// subcommands, which use commonJoinFlags + commonRemoteFlags instead
// of commonBootstrapFlags + bootstrapCmd's per-command fields.
func joinPlanFields(role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) []ui.KV {
	pairs := []ui.KV{
		{Key: "Role", Value: string(role)},
		{Key: "Cluster target", Value: flags.ClusterTarget},
		{Key: "Cluster name", Value: clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget)},
		{Key: "Node name", Value: flags.NodeName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", remoteFlags.SSHUser, remoteFlags.Host, remoteFlags.SSHPort)},
		{Key: "Operator keys", Value: keysSummary(flags.OperatorKey, flags.OperatorFiles)},
		{Key: "Labels", Value: joinOrNone(flags.Label)},
		{Key: "Taints", Value: joinOrNone(flags.Taint)},
		{Key: "Server URL", Value: hostOrFromCluster(flags.ServerURL)},
		{Key: "Reboot after install", Value: yesNo(!remoteFlags.NoReboot)},
	}
	if remoteFlags.TestVM != "" {
		pairs = append(pairs, ui.KV{Key: "Test VM", Value: remoteFlags.TestVM})
	}
	return pairs
}

// extraManifestRows turns a resolved []ExtraManifestObject into Table
// rows for the Plan section. Path is intentionally omitted — the
// operator's mental model is "what k8s objects am I creating", not
// "which YAML file does each line come from".
func extraManifestRows(objs []manifests.ExtraManifestObject) [][]string {
	rows := make([][]string, len(objs))
	for i, o := range objs {
		ns := o.Namespace
		if ns == "" {
			ns = "(cluster-scoped)"
		}
		rows[i] = []string{o.APIVersion + "/" + o.Kind, ns, o.Name}
	}
	return rows
}

var extraManifestHeaders = []string{"APIVERSION/KIND", "NAMESPACE", "NAME"}

func keysSummary(literal, files []string) string {
	switch {
	case len(literal) == 0 && len(files) == 0:
		return "(none — provisioner will fail without keys)"
	case len(literal) == 0:
		return fmt.Sprintf("%d file path(s)", len(files))
	case len(files) == 0:
		return fmt.Sprintf("%d literal", len(literal))
	default:
		return fmt.Sprintf("%d literal, %d file path(s)", len(literal), len(files))
	}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func hostOrAutoDetect(host string) string {
	if host == "" {
		return "(auto-detect from node primary IPv4)"
	}
	return host
}

func hostOrFromCluster(url string) string {
	if url == "" {
		return "(default — ~/.kube/k2/<cluster>/server-url, then cluster YAML VIP)"
	}
	return url
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
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

func (c *serverCmd) Run(rcx *runContext) error {
	return provisionJoinNode(rcx, nodeRoleServer, c.commonJoinFlags, c.commonRemoteFlags)
}

func (c *workerCmd) Run(rcx *runContext) error {
	return provisionJoinNode(rcx, nodeRoleWorker, c.commonJoinFlags, c.commonRemoteFlags)
}

func provisionJoinNode(rcx *runContext, role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) error {
	testTarget, err := applyProvisionTestVM(rcx.repoRoot, flags.ClusterTarget, &flags.ClusterName, &flags.NodeName, &remoteFlags.Host, &remoteFlags.SSHPort, remoteFlags.TestVM)
	if err != nil {
		return err
	}
	if flags.NodeName == "" {
		return fmt.Errorf("missing node name; pass --node-name or --test-vm")
	}
	_ = testTarget // currently unused for join nodes; kept for future use

	client := remote.Client{
		Host:   remoteFlags.Host,
		Port:   remoteFlags.SSHPort,
		User:   remoteFlags.SSHUser,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Logger: logf,
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	reporter.SetInterruptCancel(cancel)
	defer reporter.SetInterruptCancel(nil)

	var (
		metadata  render.ImageMetadata
		bundle    joinBundle
		localDir  string
		remoteDir string
	)

	wf := ui.NewWorkflow(reporter)

	// ---- plan + confirm -------------------------------------------------

	wf.Section("Plan")
	wf.KeyValues(joinPlanFields(role, flags, remoteFlags)...)
	wf.Confirm("Proceed with provisioning? [y/N]", "").Unless(remoteFlags.Yes)

	// ---- prelude --------------------------------------------------------

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

	// ---- render bundle --------------------------------------------------

	wf.Section("Render bundle")
	wf.Step("render-join-bundle", func(ctx context.Context) error {
		var err error
		bundle, err = buildJoinBundle(rcx.repoRoot, role, flags, metadata)
		return err
	})

	wf.Step("stage-bundle-locally", func(ctx context.Context) error {
		var err error
		localDir, err = os.MkdirTemp("", "k2-tools-"+string(role)+"-*")
		if err != nil {
			return err
		}
		wf.Defer(func() { _ = os.RemoveAll(localDir) })
		return writeJoinBundle(localDir, role, bundle)
	})

	wf.KeyValuesFn(func() []ui.KV {
		return []ui.KV{
			{Key: "Staging dir", Value: localDir},
		}
	})

	// ---- upload + install ----------------------------------------------

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

	// ---- post-reboot (only when reboot was triggered) -------------------

	postInstall := !remoteFlags.NoReboot

	wf.Section("Post-reboot").Unless(!postInstall)

	wf.Shell("Wait for SSH after reboot", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		// 10s grace period so the kernel has time to teardown the SSH
		// daemon before WaitForAuth starts probing; without this gap
		// the auth probe sometimes succeeds against the still-running
		// pre-reboot sshd and we miss the actual reboot.
		time.Sleep(10 * time.Second)
		if err := client.WaitForAuth(5 * time.Minute); err != nil {
			return err
		}
		sh.Successf("%s node %s accepted SSH", role, flags.NodeName)
		return nil
	}).Unless(!postInstall)

	wf.Shell("Verify provisioning", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return verifyRemoteProvisioning(&client, string(role)+" node "+flags.NodeName, joinVerificationScript(flags.NodeName, role), 5*time.Minute)
	}).Unless(!postInstall)

	wf.Shell("Mark worker as Longhorn storage node", func(ctx context.Context, sh ui.Step) error {
		if err := markLonghornStorageWorker(ctx, clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget), flags.NodeName, sh); err != nil {
			return err
		}
		sh.Successf("marked %s for Longhorn replica storage (%s)", flags.NodeName, longhornStorageNodeTag)
		return nil
	}).Unless(!postInstall || role != nodeRoleWorker)

	wf.Shell("Harden default access", func(ctx context.Context, sh ui.Step) error {
		defer client.SwapIO(sh)()
		return hardenRemoteDefaultAccess(&client)
	}).Unless(!postInstall)

	// ---- final banner ---------------------------------------------------

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

	return wf.Execute(parent)
}

func buildBundle(repoRoot string, flags commonBootstrapFlags, metadata render.ImageMetadata) (bundle, error) {
	logf("loading cluster config clusters/%s.yaml", flags.ClusterTarget)
	cfg, err := clusterconfig.Load(repoRoot, flags.ClusterTarget)
	if err != nil {
		return bundle{}, err
	}
	if err := rejectLonghornNodeLabels("bootstrap", flags.Label); err != nil {
		return bundle{}, err
	}
	if flags.testKubeVIP != "" {
		applyTestKubeVIP(&cfg, flags.testKubeVIP)
	}
	if flags.ClusterName == "" {
		flags.ClusterName = flags.ClusterTarget
	}
	logf("using cluster name %s", flags.ClusterName)
	logf("loading operator SSH keys")
	operatorKeys, err := keys.Load(flags.OperatorKey, flags.OperatorFiles)
	if err != nil {
		return bundle{}, err
	}
	logf("loaded %d operator SSH key(s)", len(operatorKeys))
	logf("rendering k3s cluster config")
	clusterConfig, err := render.ClusterConfig(cfg)
	if err != nil {
		return bundle{}, err
	}
	logf("rendering k3s bootstrap config")
	bootstrapConfig, err := render.BootstrapConfig(render.BootstrapInput{
		Cluster:       cfg,
		NodeName:      flags.NodeName,
		Labels:        flags.Label,
		Taints:        flags.Taint,
		ImageMetadata: metadata,
	})
	if err != nil {
		return bundle{}, err
	}
	logf("assembling bootstrap manifests from %s", cfg.DeployDir(repoRoot))
	bootstrapManifests, err := manifests.Bootstrap(repoRoot, cfg, manifests.BootstrapOptions{
		ExtraManifestPatterns: flags.ExtraManifests,
		CiliumAPIHost:         flags.BootstrapAPIHost,
	})
	if err != nil {
		return bundle{}, err
	}
	rootArgoApp, err := manifests.RootArgoApp(cfg)
	if err != nil {
		return bundle{}, err
	}
	return bundle{
		ClusterConfig:   clusterConfig,
		BootstrapConfig: bootstrapConfig,
		Activation:      render.ActivationCloudConfig(flags.NodeName, operatorKeys),
		AuthorizedKeys:  render.AuthorizedKeys(operatorKeys),
		Manifests:       bootstrapManifests,
		RootArgoApp:     rootArgoApp,
	}, nil
}

func applyProvisionTestVM(repoRoot string, clusterTarget string, clusterName *string, nodeName *string, host *string, sshPort *int, vmID string) (testVMProvisionTarget, error) {
	if vmID == "" {
		if *host == "" {
			return testVMProvisionTarget{}, fmt.Errorf("missing SSH host; pass --host or --test-vm")
		}
		return testVMProvisionTarget{}, nil
	}

	target, err := testvm.ResolveProvisionTarget(repoRoot, vmID)
	if err != nil {
		return testVMProvisionTarget{}, err
	}
	if *clusterName == "" {
		*clusterName = clusterTarget + "-vmtest"
	}
	if *nodeName == "" {
		*nodeName = vmID
	}
	*host = target.Host
	*sshPort = target.Port

	out := testVMProvisionTarget{Enabled: true, GuestIP: target.GuestIPv4.Address}
	if target.GuestIPv4.Address != "" {
		vip, err := testKubeVIP(target.GuestIPv4.Address, target.GuestIPv4.Prefix)
		if err != nil {
			return testVMProvisionTarget{}, err
		}
		out.KubeVIP = vip
	}
	return out, nil
}

func applyTestKubeVIP(cfg *clusterconfig.Config, vip string) {
	cfg.Kubernetes.API = vip
}

func testKubeVIP(nodeIP string, prefix int) (string, error) {
	parsed := net.ParseIP(nodeIP).To4()
	if parsed == nil {
		return "", fmt.Errorf("test VM node address %q is not IPv4", nodeIP)
	}
	if prefix <= 0 || prefix >= 31 {
		return "", fmt.Errorf("test VM node address %s has unsupported prefix %d", nodeIP, prefix)
	}

	ip := binary.BigEndian.Uint32(parsed)
	mask := uint32(0xffffffff) << (32 - prefix)
	network := ip & mask
	broadcast := network | ^mask
	candidate := broadcast - 1
	if candidate == ip {
		candidate--
	}
	if candidate <= network {
		return "", fmt.Errorf("could not choose test VM kube-vip address in %s/%d", nodeIP, prefix)
	}

	var out [4]byte
	binary.BigEndian.PutUint32(out[:], candidate)
	return net.IP(out[:]).String(), nil
}

func buildJoinBundle(repoRoot string, role nodeRole, flags commonJoinFlags, metadata render.ImageMetadata) (joinBundle, error) {
	logf("loading cluster config clusters/%s.yaml", flags.ClusterTarget)
	cfg, err := clusterconfig.Load(repoRoot, flags.ClusterTarget)
	if err != nil {
		return joinBundle{}, err
	}
	clusterName := flags.ClusterName
	if clusterName == "" {
		clusterName = flags.ClusterTarget
	}
	logf("using cluster name %s", clusterName)
	logf("loading operator SSH keys")
	operatorKeys, err := keys.Load(flags.OperatorKey, flags.OperatorFiles)
	if err != nil {
		return joinBundle{}, err
	}
	logf("loaded %d operator SSH key(s)", len(operatorKeys))

	serverURL, err := resolveJoinServerURL(cfg, clusterName, flags.ServerURL)
	if err != nil {
		return joinBundle{}, err
	}
	tokenName := "agent-token"
	if role == nodeRoleServer {
		tokenName = "server-token"
	}
	token, err := readClusterCredential(clusterName, tokenName)
	if err != nil {
		return joinBundle{}, err
	}

	logf("rendering k3s %s join config", role)
	nodeLabels, err := nodeLabelsForRole(role, flags.Label)
	if err != nil {
		return joinBundle{}, err
	}
	joinConfig, err := render.JoinConfig(render.JoinInput{
		NodeName:      flags.NodeName,
		ServerURL:     serverURL,
		Token:         token,
		Labels:        nodeLabels,
		Taints:        flags.Taint,
		ImageMetadata: metadata,
		ControlPlane:  role == nodeRoleServer,
	})
	if err != nil {
		return joinBundle{}, err
	}

	var clusterConfig []byte
	if role == nodeRoleServer {
		logf("rendering k3s cluster config")
		clusterConfig, err = render.ClusterConfig(cfg)
		if err != nil {
			return joinBundle{}, err
		}
	}

	activation := render.AgentActivationCloudConfig(flags.NodeName, operatorKeys)
	if role == nodeRoleServer {
		activation = render.ServerActivationCloudConfig(flags.NodeName, operatorKeys)
	}

	return joinBundle{
		ClusterConfig:  clusterConfig,
		JoinConfig:     joinConfig,
		Activation:     activation,
		AuthorizedKeys: render.AuthorizedKeys(operatorKeys),
	}, nil
}

func writeBundle(dir string, bundle bundle) error {
	files := map[string][]byte{
		"20-k2-cluster.yaml":          bundle.ClusterConfig,
		"30-k2-bootstrap.yaml":        bundle.BootstrapConfig,
		"99-k2-k3s-bootstrap.yaml":    bundle.Activation,
		"operator_authorized_keys":    bundle.AuthorizedKeys,
		"k2-bootstrap.k8s.yaml":       bundle.Manifests,
		"k2-root-argocd-app.k8s.yaml": bundle.RootArgoApp,
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeJoinBundle(dir string, role nodeRole, bundle joinBundle) error {
	files := map[string][]byte{
		"30-k2-" + string(role) + ".yaml":     bundle.JoinConfig,
		"99-k2-k3s-" + string(role) + ".yaml": bundle.Activation,
		"operator_authorized_keys":            bundle.AuthorizedKeys,
	}
	if role == nodeRoleServer {
		files["20-k2-cluster.yaml"] = bundle.ClusterConfig
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func logf(format string, args ...any) {
	reporter.Infof(format, args...)
}

func successf(format string, args ...any) {
	reporter.Successf(format, args...)
}

func warnf(format string, args ...any) {
	reporter.Warnf(format, args...)
}

func readRemoteMetadata(client *remote.Client) (render.ImageMetadata, error) {
	data, err := client.ReadFile("/usr/share/k2/image-build/metadata.yaml")
	if err != nil {
		return render.ImageMetadata{}, fmt.Errorf("read remote image metadata: %w", err)
	}
	metadata, err := render.DecodeImageMetadata(data)
	if err != nil {
		return render.ImageMetadata{}, err
	}
	if metadata.Target == "" || metadata.Arch == "" || metadata.Hardware == "" {
		return render.ImageMetadata{}, fmt.Errorf("remote image metadata is incomplete; target, arch, and hardware are required")
	}
	return metadata, nil
}

func markLonghornStorageWorker(ctx context.Context, clusterName string, nodeName string, out ui.Step) error {
	kubeconfigPath, err := kubeconfigPathFor(clusterName)
	if err != nil {
		return err
	}
	kc := kubectl.New(kubeconfigPath)
	kc.Stderr = out
	kc.Logger = logf
	if err := kc.Available(); err != nil {
		return fmt.Errorf("%w; install kubectl + ensure it's on PATH", err)
	}
	if err := kc.LabelNode(ctx, nodeName, longhornStorageNodeLabel); err != nil {
		return err
	}
	if err := kc.AnnotateNode(ctx, nodeName, longhornStorageNodeTagsAnnotation); err != nil {
		return err
	}
	return nil
}

func detectBootstrapAPIHost(client *remote.Client) (string, error) {
	script := strings.Join([]string{
		`host="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p' | head -n1)"`,
		`if [ -z "$host" ]; then host="$(hostname -I 2>/dev/null | awk '{print $1}')"; fi`,
		`printf '%s\n' "$host"`,
	}, "; ")
	out, err := client.Capture(script)
	if err != nil {
		return "", fmt.Errorf("detect bootstrap API host: %w", err)
	}
	host := strings.TrimSpace(string(out))
	if parsed := net.ParseIP(host); parsed == nil || parsed.To4() == nil {
		return "", fmt.Errorf("detected bootstrap API host %q is not an IPv4 address; pass --bootstrap-api-host", host)
	}
	return host, nil
}

func harvestBootstrapCredentials(client *remote.Client, cfg clusterconfig.Config, clusterName string) error {
	logf("waiting for node to reboot and accept SSH")
	time.Sleep(10 * time.Second)
	if err := client.WaitForAuth(5 * time.Minute); err != nil {
		return err
	}

	logf("waiting for k3s credentials on bootstrap node")
	if err := waitForK3sCredentials(client, 5*time.Minute); err != nil {
		return err
	}

	logf("reading kubeconfig and k3s tokens from bootstrap node")
	rawKubeconfig, err := client.ReadSudoFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return err
	}
	serverToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/token")
	if err != nil {
		return err
	}
	nodeToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return err
	}
	agentToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/agent-token")
	if err != nil {
		return err
	}

	rewrittenKubeconfig, err := kubeconfig.RewriteServer(rawKubeconfig, cfg.APIServerURL())
	if err != nil {
		return err
	}

	dir, err := clusterCredentialsDir(clusterName)
	if err != nil {
		return err
	}
	logf("writing cluster credentials to %s", dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	files := map[string][]byte{
		"kubeconfig":   rewrittenKubeconfig,
		"server-token": []byte(strings.TrimSpace(string(serverToken)) + "\n"),
		"node-token":   []byte(strings.TrimSpace(string(nodeToken)) + "\n"),
		"agent-token":  []byte(strings.TrimSpace(string(agentToken)) + "\n"),
		"server-url":   []byte(cfg.APIServerURL() + "\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	successf("cluster credentials written; use KUBECONFIG=%s", filepath.Join(dir, "kubeconfig"))
	return nil
}

func waitForK3sCredentials(client *remote.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Check(strings.Join([]string{
			"sudo test -s /etc/rancher/k3s/k3s.yaml",
			"sudo test -s /var/lib/rancher/k3s/server/token",
			"sudo test -s /var/lib/rancher/k3s/server/node-token",
			"sudo test -s /var/lib/rancher/k3s/server/agent-token",
		}, " && "))
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for k3s credentials: %w", lastErr)
		}
		time.Sleep(5 * time.Second)
	}
}

func patchRemoteKubeVIP(client *remote.Client, vip string, timeout time.Duration) error {
	logf("patching kube-vip VIP address for test VM bootstrap to %s", vip)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Run(strings.Join([]string{
			"sudo kubectl -n kube-vip get daemonset kube-vip >/dev/null",
			fmt.Sprintf("sudo kubectl -n kube-vip set env daemonset/kube-vip vip_address=%s >/dev/null", vip),
			"sudo kubectl -n kube-vip rollout status daemonset/kube-vip --timeout=120s >/dev/null",
		}, " && "))
		if err == nil {
			successf("kube-vip VIP patched to %s", vip)
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out patching kube-vip VIP address: %w", lastErr)
		}
		time.Sleep(5 * time.Second)
	}
}

func applyRootArgoApp(client *remote.Client, timeout time.Duration) error {
	logf("applying root Argo CD app manifest")
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Run(rootArgoAppApplyScript(remoteRootArgoAppManifestPath))
		if err == nil {
			successf("root Argo CD app manifest applied")
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out applying root Argo CD app manifest: %w", lastErr)
		}
		time.Sleep(5 * time.Second)
	}
}

func rootArgoAppApplyScript(manifestPath string) string {
	return strings.Join([]string{
		"set -eu",
		"sudo kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=30s >/dev/null",
		fmt.Sprintf("sudo kubectl apply -f %s", shellQuote(manifestPath)),
	}, "\n")
}

func verifyRemoteProvisioning(client *remote.Client, description string, script string, timeout time.Duration) error {
	logf("verifying %s provisioning", description)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Run(script)
		if err == nil {
			successf("%s provisioning verified", description)
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out verifying %s provisioning: %w", description, lastErr)
		}
		time.Sleep(5 * time.Second)
	}
}

func hardenRemoteDefaultAccess(client *remote.Client) error {
	logf("hardening default kairos access")
	script := strings.Join([]string{
		"set -eu",
		"if sudo test -f /oem/90_custom.yaml; then sudo mv /oem/90_custom.yaml /oem/90_custom.yaml.k2-disabled; fi",
		"sudo passwd -l kairos",
		"sudo test ! -e /oem/90_custom.yaml",
	}, "\n")
	if err := client.Run(script); err != nil {
		return fmt.Errorf("harden default kairos access: %w", err)
	}
	successf("default kairos access hardened")
	return nil
}

func bootstrapVerificationScript(nodeName string) string {
	var buf bytes.Buffer
	writeVerificationPrelude(&buf, nodeName)
	fmt.Fprintf(&buf, "verify 'server invariant config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
	fmt.Fprintf(&buf, "verify 'cluster config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n")
	fmt.Fprintf(&buf, "verify 'bootstrap config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml\n")
	fmt.Fprintf(&buf, "verify 'bootstrap activation installed' sudo test -s /oem/99-k2-k3s-bootstrap.yaml\n")
	fmt.Fprintf(&buf, "verify 'k3s service enabled' systemctl is-enabled --quiet k3s\n")
	fmt.Fprintf(&buf, "verify 'k3s service active' systemctl is-active --quiet k3s\n")
	fmt.Fprintf(&buf, "verify 'k3s kubeconfig exists' sudo test -s /etc/rancher/k3s/k3s.yaml\n")
	fmt.Fprintf(&buf, "verify 'server token exists' sudo test -s /var/lib/rancher/k3s/server/token\n")
	fmt.Fprintf(&buf, "verify 'node token exists' sudo test -s /var/lib/rancher/k3s/server/node-token\n")
	fmt.Fprintf(&buf, "verify 'agent token exists' sudo test -s /var/lib/rancher/k3s/server/agent-token\n")
	fmt.Fprintf(&buf, "verify 'root Argo CD app manifest staged' sudo test -s %s\n", remoteRootArgoAppManifestPath)
	fmt.Fprintf(&buf, "verify 'root Argo CD Application applied' sudo kubectl -n argocd get application k2 >/dev/null\n")
	writeServerPackagedManifestChecks(&buf)
	return buf.String()
}

func joinVerificationScript(nodeName string, role nodeRole) string {
	var buf bytes.Buffer
	writeVerificationPrelude(&buf, nodeName)
	configFile := "30-k2-" + string(role) + ".yaml"
	activationFile := "99-k2-k3s-" + string(role) + ".yaml"
	fmt.Fprintf(&buf, "verify '%s join config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/%s\n", role, configFile)
	fmt.Fprintf(&buf, "verify '%s activation installed' sudo test -s /oem/%s\n", role, activationFile)
	if role == nodeRoleServer {
		fmt.Fprintf(&buf, "verify 'server invariant config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
		fmt.Fprintf(&buf, "verify 'cluster config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n")
		fmt.Fprintf(&buf, "verify 'k3s service enabled' systemctl is-enabled --quiet k3s\n")
		fmt.Fprintf(&buf, "verify 'k3s service active' systemctl is-active --quiet k3s\n")
		fmt.Fprintf(&buf, "verify 'k3s kubeconfig exists' sudo test -s /etc/rancher/k3s/k3s.yaml\n")
		writeServerPackagedManifestChecks(&buf)
	} else {
		fmt.Fprintf(&buf, "verify 'server invariant config absent on worker' sudo test ! -e /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
		fmt.Fprintf(&buf, "verify 'cluster config absent on worker' sudo test ! -e /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n")
		fmt.Fprintf(&buf, "verify 'k3s-agent service enabled' systemctl is-enabled --quiet k3s-agent\n")
		fmt.Fprintf(&buf, "verify 'k3s-agent service active' systemctl is-active --quiet k3s-agent\n")
	}
	return buf.String()
}

func writeVerificationPrelude(buf *bytes.Buffer, nodeName string) {
	fmt.Fprintf(buf, "set -eu\n")
	fmt.Fprintf(buf, "verify() { label=\"$1\"; shift; echo \"k2-tools: verify: ${label}\"; \"$@\"; }\n")
	fmt.Fprintf(buf, "verify 'hostname set' test \"$(hostname)\" = %s\n", shellQuote(nodeName))
	fmt.Fprintf(buf, "verify 'operator authorized keys installed' sudo test -s /home/kairos/.ssh/authorized_keys\n")
}

func writeServerPackagedManifestChecks(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "verify 'traefik packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip\n")
	fmt.Fprintf(buf, "verify 'local-storage packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip\n")
	fmt.Fprintf(buf, "verify 'metrics-server packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip\n")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func clusterCredentialsDir(clusterName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "k2", clusterName), nil
}

func readClusterCredential(clusterName string, name string) (string, error) {
	dir, err := clusterCredentialsDir(clusterName)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cluster credential %s: %w", path, err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("cluster credential %s is empty", path)
	}
	return value, nil
}

func resolveJoinServerURL(cfg clusterconfig.Config, clusterName string, override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	value, err := readClusterCredential(clusterName, "server-url")
	if err == nil {
		return value, nil
	}
	warnf("could not read saved server-url for cluster %s: %v; using cluster config API VIP URL", clusterName, err)
	return cfg.APIServerURL(), nil
}

func installScript(remoteDir string, nodeName string, noReboot bool) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: installing bootstrap files'\n")
	fmt.Fprintf(&buf, "sudo mkdir -p /etc/rancher/k3s/config.yaml.d /var/lib/rancher/k3s/server/manifests /oem /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: activating k3s server invariants'\n")
	fmt.Fprintf(&buf, "sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: disabling unwanted k3s packaged manifests'\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: installing cluster and bootstrap config'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/30-k2-bootstrap.yaml /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-tools: installing Kairos k3s activation cloud-config'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-tools: installing bootstrap manifest bundle'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/k2-bootstrap.k8s.yaml /var/lib/rancher/k3s/server/manifests/k2-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-tools: staging root Argo CD app manifest'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/k2-root-argocd-app.k8s.yaml %s\n", remoteDir, remoteRootArgoAppManifestPath)
	fmt.Fprintf(&buf, "echo 'k2-tools: installing operator SSH keys'\n")
	fmt.Fprintf(&buf, "sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "sudo install -o kairos -g kairos -m 0600 %q/operator_authorized_keys /home/kairos/.ssh/authorized_keys\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-tools: enabling k3s service'\n")
	fmt.Fprintf(&buf, "sudo systemctl enable k3s\n")
	if !noReboot {
		fmt.Fprintf(&buf, "echo 'k2-tools: rebooting node'\n")
		fmt.Fprintf(&buf, "sudo reboot\n")
	}
	return buf.String()
}

func joinInstallScript(remoteDir string, nodeName string, role nodeRole, noReboot bool) string {
	var buf bytes.Buffer
	configFile := "30-k2-" + string(role) + ".yaml"
	activationFile := "99-k2-k3s-" + string(role) + ".yaml"
	service := "k3s-agent"
	if role == nodeRoleServer {
		service = "k3s"
	}

	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: installing %s files'\n", role)
	fmt.Fprintf(&buf, "sudo mkdir -p /etc/rancher/k3s/config.yaml.d /oem /home/kairos/.ssh\n")
	if role == nodeRoleServer {
		fmt.Fprintf(&buf, "sudo mkdir -p /var/lib/rancher/k3s/server/manifests\n")
		fmt.Fprintf(&buf, "echo 'k2-tools: activating k3s server invariants'\n")
		fmt.Fprintf(&buf, "sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
		fmt.Fprintf(&buf, "echo 'k2-tools: disabling unwanted k3s packaged manifests'\n")
		fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip\n")
		fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip\n")
		fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip\n")
		fmt.Fprintf(&buf, "echo 'k2-tools: installing cluster config'\n")
		fmt.Fprintf(&buf, "sudo install -m 0644 %q/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n", remoteDir)
	}
	fmt.Fprintf(&buf, "echo 'k2-tools: installing %s join config'\n", role)
	fmt.Fprintf(&buf, "sudo install -m 0600 %q/%s /etc/rancher/k3s/config.yaml.d/%s\n", remoteDir, configFile, configFile)
	fmt.Fprintf(&buf, "echo 'k2-tools: installing Kairos k3s activation cloud-config'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/%s /oem/%s\n", remoteDir, activationFile, activationFile)
	fmt.Fprintf(&buf, "echo 'k2-tools: installing operator SSH keys'\n")
	fmt.Fprintf(&buf, "sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "sudo install -o kairos -g kairos -m 0600 %q/operator_authorized_keys /home/kairos/.ssh/authorized_keys\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-tools: enabling %s service'\n", service)
	fmt.Fprintf(&buf, "sudo systemctl enable %s\n", service)
	if !noReboot {
		fmt.Fprintf(&buf, "echo 'k2-tools: rebooting node'\n")
		fmt.Fprintf(&buf, "sudo reboot\n")
	}
	return buf.String()
}
