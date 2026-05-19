package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/kairos/provision/internal/clusterconfig"
	"github.com/wyvernzora/k2/kairos/provision/internal/keys"
	"github.com/wyvernzora/k2/kairos/provision/internal/kubeconfig"
	"github.com/wyvernzora/k2/kairos/provision/internal/manifests"
	"github.com/wyvernzora/k2/kairos/provision/internal/prereq"
	"github.com/wyvernzora/k2/kairos/provision/internal/remote"
	"github.com/wyvernzora/k2/kairos/provision/internal/render"
	"github.com/wyvernzora/k2/kairos/provision/internal/workspace"
)

type cli struct {
	RepoRoot string `name:"repo-root" help:"Repository root. Defaults to auto-detection." type:"path"`

	Bootstrap bootstrapCmd `cmd:"" help:"Provision the first K3s server over SSH."`
	Render    renderCmd    `cmd:"" help:"Render provisioning files locally."`
}

type renderCmd struct {
	Bootstrap renderBootstrapCmd `cmd:"" help:"Render bootstrap provisioning files."`
}

type commonBootstrapFlags struct {
	ClusterTarget    string   `name:"cluster-target" required:"" help:"Cluster config/deploy target, such as v3."`
	ClusterName      string   `name:"cluster-name" help:"Local cluster instance name. Defaults to cluster-target."`
	NodeName         string   `name:"node-name" required:"" help:"Kubernetes node name."`
	OperatorKey      []string `name:"operator-key" help:"Literal ssh-ed25519 operator public key. Repeatable."`
	OperatorFiles    []string `name:"operator-key-file" help:"File containing literal operator public keys. Repeatable." type:"path"`
	Label            []string `name:"label" help:"Additional K3s node-label value. Repeatable."`
	Taint            []string `name:"taint" help:"Additional K3s node-taint value. Repeatable."`
	BootstrapAPIHost string   `name:"bootstrap-api-host" help:"Kubernetes API host for bootstrap-only Cilium manifests. Bootstrap provisioning auto-detects the node IP when omitted."`

	OnePasswordTokenFile string `name:"onepassword-token-file" help:"Optional 1Password service account token file to include as bootstrap Secret." type:"path"`
	SecretNamespace      string `name:"bootstrap-secret-namespace" default:"secrets" help:"Namespace for optional bootstrap Secret."`
	SecretName           string `name:"bootstrap-secret-name" default:"onepassword-service-account-token" help:"Name for optional bootstrap Secret."`
}

type bootstrapCmd struct {
	commonBootstrapFlags

	Host     string `name:"host" required:"" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" help:"Install files and enable k3s, but do not reboot."`
}

type renderBootstrapCmd struct {
	commonBootstrapFlags

	OutputDir string `name:"output-dir" required:"" help:"Directory to write rendered files into." type:"path"`
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var app cli
	parser, err := kong.New(&app, kong.Name("k2-provision"), kong.UsageOnError())
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
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
	logf("wrote bootstrap bundle to %s", c.OutputDir)
	return nil
}

func (c *bootstrapCmd) Run(ctx *runContext) error {
	logf("checking local command prerequisites")
	if err := prereq.Require("ssh", "scp"); err != nil {
		return err
	}

	client := remote.Client{
		Host:   c.Host,
		Port:   c.SSHPort,
		User:   c.SSHUser,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
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
	localDir, err := os.MkdirTemp("", "k2-provision-bootstrap-*")
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
	logf("uploaded bootstrap bundle to %s", remoteDir)
	if err := client.Run(installScript(remoteDir, c.NodeName, c.NoReboot)); err != nil {
		return err
	}
	if c.NoReboot {
		logf("remote install complete; reboot skipped")
		logf("skipping credential harvest because --no-reboot leaves k3s stopped")
	} else {
		logf("remote install complete; node should be rebooting")
		cfg, err := clusterconfig.Load(ctx.repoRoot, c.ClusterTarget)
		if err != nil {
			return err
		}
		clusterName := c.ClusterName
		if clusterName == "" {
			clusterName = c.ClusterTarget
		}
		if err := harvestBootstrapCredentials(&client, cfg, clusterName); err != nil {
			return err
		}
	}
	return nil
}

func buildBundle(repoRoot string, flags commonBootstrapFlags, metadata render.ImageMetadata) (bundle, error) {
	logf("loading cluster config clusters/%s.yaml", flags.ClusterTarget)
	cfg, err := clusterconfig.Load(repoRoot, flags.ClusterTarget)
	if err != nil {
		return bundle{}, err
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

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "k2-provision: "+format+"\n", args...)
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
		"server-url":   []byte(cfg.APIServerURL() + "\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	logf("cluster credentials written; use KUBECONFIG=%s", filepath.Join(dir, "kubeconfig"))
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

func clusterCredentialsDir(clusterName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "k2", clusterName), nil
}

func installScript(remoteDir string, nodeName string, noReboot bool) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: installing bootstrap files'\n")
	fmt.Fprintf(&buf, "sudo mkdir -p /etc/rancher/k3s/config.yaml.d /var/lib/rancher/k3s/server/manifests /oem /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: setting hostname'\n")
	fmt.Fprintf(&buf, "sudo hostnamectl set-hostname %q\n", nodeName)
	fmt.Fprintf(&buf, "echo 'k2-provision: activating k3s server invariants'\n")
	fmt.Fprintf(&buf, "sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: disabling unwanted k3s packaged manifests'\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip\n")
	fmt.Fprintf(&buf, "sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: installing cluster and bootstrap config'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/30-k2-bootstrap.yaml /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-provision: installing Kairos k3s activation cloud-config'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "if sudo test -f /oem/90_custom.yaml; then echo 'k2-provision: disabling stock Kairos default credentials cloud-config'; sudo mv /oem/90_custom.yaml /oem/90_custom.yaml.k2-disabled; fi\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: installing bootstrap manifest bundle'\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 %q/k2-bootstrap.k8s.yaml /var/lib/rancher/k3s/server/manifests/k2-bootstrap.yaml\n", remoteDir)
	fmt.Fprintf(&buf, "echo 'k2-provision: installing operator SSH keys'\n")
	fmt.Fprintf(&buf, "sudo install -o kairos -g kairos -m 0600 %q/operator_authorized_keys /home/kairos/.ssh/authorized_keys\n", remoteDir)
	fmt.Fprintf(&buf, "sudo chmod 0700 /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: locking default kairos password'\n")
	fmt.Fprintf(&buf, "sudo passwd -l kairos\n")
	fmt.Fprintf(&buf, "echo 'k2-provision: enabling k3s service'\n")
	fmt.Fprintf(&buf, "sudo systemctl enable k3s\n")
	if !noReboot {
		fmt.Fprintf(&buf, "echo 'k2-provision: rebooting node'\n")
		fmt.Fprintf(&buf, "sudo reboot\n")
	}
	return buf.String()
}
