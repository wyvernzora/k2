package main

import (
	"bytes"
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

	OnePasswordTokenFile string `name:"onepassword-token-file" env:"K2_PROVISION_ONEPASSWORD_TOKEN_FILE" help:"Optional 1Password service account token file to include as bootstrap Secret." type:"path"`
	SecretNamespace      string `name:"bootstrap-secret-namespace" env:"K2_PROVISION_BOOTSTRAP_SECRET_NAMESPACE" default:"secrets" help:"Namespace for optional bootstrap Secret."`
	SecretName           string `name:"bootstrap-secret-name" env:"K2_PROVISION_BOOTSTRAP_SECRET_NAME" default:"onepassword-service-account-token" help:"Name for optional bootstrap Secret."`

	testKubeVIP string
}

type bootstrapCmd struct {
	commonBootstrapFlags

	TestVM   string `name:"test-vm" env:"K2_PROVISION_TEST_VM" help:"Provision the local test VM id, defaulting host and cluster-name for VM swarm tests."`
	Host     string `name:"host" env:"K2_PROVISION_HOST" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" env:"K2_PROVISION_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" env:"K2_PROVISION_SSH_USER" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" env:"K2_PROVISION_NO_REBOOT" help:"Install files and enable k3s, but do not reboot."`
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

func (c *bootstrapCmd) Run(ctx *runContext) error {
	testTarget, err := c.prepareTestVM(ctx)
	if err != nil {
		return err
	}

	client := remote.Client{
		Host:   c.Host,
		Port:   c.SSHPort,
		User:   c.SSHUser,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Logger: logf,
	}

	logf("provision bootstrap node %s via %s", c.NodeName, client.Address())
	logf("reading remote image metadata")
	metadata, err := readRemoteMetadata(&client)
	if err != nil {
		return fmt.Errorf("%w; rebuild the image with baked metadata support", err)
	}
	if c.BootstrapAPIHost == "" {
		logf("detecting bootstrap API host")
		c.BootstrapAPIHost, err = detectBootstrapAPIHost(&client)
		if err != nil {
			return err
		}
		logf("using bootstrap API host %s", c.BootstrapAPIHost)
	}

	logf("rendering bootstrap bundle for target %s", c.ClusterTarget)
	bundle, err := buildBundle(ctx.repoRoot, c.commonBootstrapFlags, metadata)
	if err != nil {
		return err
	}

	logf("creating local staging directory")
	localDir, err := os.MkdirTemp("", "k2-tools-bootstrap-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(localDir)
	if err := writeBundle(localDir, bundle); err != nil {
		return err
	}
	logf("staged bootstrap bundle in %s", localDir)

	remoteDir, err := client.UploadDir(localDir)
	if err != nil {
		return err
	}
	successf("uploaded bootstrap bundle to %s", remoteDir)
	if err := client.RunAllowDisconnect(installScript(remoteDir, c.NodeName, c.NoReboot)); err != nil {
		return err
	}
	if c.NoReboot {
		successf("remote install complete; reboot skipped")
		logf("skipping credential harvest because --no-reboot leaves k3s stopped")
		return nil
	}
	return c.completeAfterBootstrapReboot(ctx, &client, testTarget)
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

func (c *bootstrapCmd) completeAfterBootstrapReboot(ctx *runContext, client *remote.Client, testTarget testVMProvisionTarget) error {
	successf("remote install complete; node should be rebooting")
	cfg, err := clusterconfig.Load(ctx.repoRoot, c.ClusterTarget)
	if err != nil {
		return err
	}
	if testTarget.KubeVIP != "" {
		applyTestKubeVIP(&cfg, testTarget.KubeVIP)
	}
	clusterName := c.ClusterName
	if clusterName == "" {
		clusterName = c.ClusterTarget
	}
	if err := harvestBootstrapCredentials(client, cfg, clusterName); err != nil {
		return err
	}
	if testTarget.KubeVIP != "" {
		if err := patchRemoteKubeVIP(client, testTarget.KubeVIP, 3*time.Minute); err != nil {
			return err
		}
	}
	if err := verifyRemoteProvisioning(client, "bootstrap node "+c.NodeName, bootstrapVerificationScript(c.NodeName), 5*time.Minute); err != nil {
		return err
	}
	return hardenRemoteDefaultAccess(client)
}

func (c *serverCmd) Run(ctx *runContext) error {
	return provisionJoinNode(ctx, nodeRoleServer, c.commonJoinFlags, c.commonRemoteFlags)
}

func (c *workerCmd) Run(ctx *runContext) error {
	return provisionJoinNode(ctx, nodeRoleWorker, c.commonJoinFlags, c.commonRemoteFlags)
}

func provisionJoinNode(ctx *runContext, role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) error {
	testTarget, err := applyProvisionTestVM(ctx.repoRoot, flags.ClusterTarget, &flags.ClusterName, &flags.NodeName, &remoteFlags.Host, &remoteFlags.SSHPort, remoteFlags.TestVM)
	if err != nil {
		return err
	}
	if flags.NodeName == "" {
		return fmt.Errorf("missing node name; pass --node-name or --test-vm")
	}
	if testTarget.Enabled {
		logf("using test VM %s: ssh %s:%d, cluster %s", remoteFlags.TestVM, remoteFlags.Host, remoteFlags.SSHPort, flags.ClusterName)
	}

	client := remote.Client{
		Host:   remoteFlags.Host,
		Port:   remoteFlags.SSHPort,
		User:   remoteFlags.SSHUser,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Logger: logf,
	}

	logf("provision %s node %s via %s", role, flags.NodeName, client.Address())
	logf("reading remote image metadata")
	metadata, err := readRemoteMetadata(&client)
	if err != nil {
		return fmt.Errorf("%w; rebuild the image with baked metadata support", err)
	}

	bundle, err := buildJoinBundle(ctx.repoRoot, role, flags, metadata)
	if err != nil {
		return err
	}

	logf("creating local staging directory")
	localDir, err := os.MkdirTemp("", "k2-tools-"+string(role)+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(localDir)
	if err := writeJoinBundle(localDir, role, bundle); err != nil {
		return err
	}
	logf("staged %s bundle in %s", role, localDir)

	remoteDir, err := client.UploadDir(localDir)
	if err != nil {
		return err
	}
	successf("uploaded %s bundle to %s", role, remoteDir)
	if err := client.RunAllowDisconnect(joinInstallScript(remoteDir, flags.NodeName, role, remoteFlags.NoReboot)); err != nil {
		return err
	}
	if remoteFlags.NoReboot {
		successf("remote install complete; reboot skipped")
		return nil
	}
	successf("remote install complete; node should be rebooting")
	logf("waiting for node to reboot and accept SSH")
	time.Sleep(10 * time.Second)
	if err := client.WaitForAuth(5 * time.Minute); err != nil {
		return err
	}
	successf("%s node %s accepted SSH after reboot", role, flags.NodeName)
	if err := verifyRemoteProvisioning(&client, string(role)+" node "+flags.NodeName, joinVerificationScript(flags.NodeName, role), 5*time.Minute); err != nil {
		return err
	}
	if err := hardenRemoteDefaultAccess(&client); err != nil {
		return err
	}
	return nil
}

func buildBundle(repoRoot string, flags commonBootstrapFlags, metadata render.ImageMetadata) (bundle, error) {
	logf("loading cluster config clusters/%s.yaml", flags.ClusterTarget)
	cfg, err := clusterconfig.Load(repoRoot, flags.ClusterTarget)
	if err != nil {
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
		OnePasswordTokenFile: flags.OnePasswordTokenFile,
		SecretNamespace:      flags.SecretNamespace,
		SecretName:           flags.SecretName,
		CiliumAPIHost:        flags.BootstrapAPIHost,
	})
	if err != nil {
		return bundle{}, err
	}
	return bundle{
		ClusterConfig:   clusterConfig,
		BootstrapConfig: bootstrapConfig,
		Activation:      render.ActivationCloudConfig(flags.NodeName, operatorKeys),
		AuthorizedKeys:  render.AuthorizedKeys(operatorKeys),
		Manifests:       bootstrapManifests,
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
	cfg.Kubernetes.API.VIP = vip
	cfg.Kubernetes.API.DNSName = ""
	for _, san := range cfg.Kubernetes.API.TLSSans {
		if san == vip {
			return
		}
	}
	cfg.Kubernetes.API.TLSSans = append(cfg.Kubernetes.API.TLSSans, vip)
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
	joinConfig, err := render.JoinConfig(render.JoinInput{
		NodeName:      flags.NodeName,
		ServerURL:     serverURL,
		Token:         token,
		Labels:        flags.Label,
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
		"20-k2-cluster.yaml":       bundle.ClusterConfig,
		"30-k2-bootstrap.yaml":     bundle.BootstrapConfig,
		"99-k2-k3s-bootstrap.yaml": bundle.Activation,
		"operator_authorized_keys": bundle.AuthorizedKeys,
		"k2-bootstrap.k8s.yaml":    bundle.Manifests,
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
		"server-url":   []byte(cfg.APIVIPURL() + "\n"),
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
	return cfg.APIVIPURL(), nil
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
