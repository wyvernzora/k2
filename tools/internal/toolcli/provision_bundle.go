package toolcli

import (
	"os"
	"path/filepath"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/keys"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/manifests"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/render"
)

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
