package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/clusterconfig"
	"github.com/wyvernzora/k2/tools/internal/shellscript"
)

func rootArgoAppApplyScript(manifestPath string) string {
	return strings.Join([]string{
		"set -eu",
		"sudo kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=30s >/dev/null",
		fmt.Sprintf("sudo kubectl apply -f %s", shellQuote(manifestPath)),
	}, "\n")
}

func bootstrapVerificationScript(nodeName string) string {
	return renderScript("bootstrap-verify.sh.tmpl", map[string]any{
		"NodeName":                nodeName,
		"RootArgoAppManifestPath": remoteRootArgoAppManifestPath,
	})
}

func joinVerificationScript(nodeName string, role nodeRole, hasNetwork bool) string {
	return renderScript("join-verify.sh.tmpl", map[string]any{
		"NodeName":       nodeName,
		"Role":           string(role),
		"ConfigFile":     "30-k2-" + string(role) + ".yaml",
		"ActivationFile": "99-k2-k3s-" + string(role) + ".yaml",
		"HasNetwork":     hasNetwork,
		"IsServer":       role == nodeRoleServer,
	})
}

func shellQuote(value string) string {
	return shellscript.Quote(value)
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
	return renderScript("install.sh.tmpl", map[string]any{
		"RemoteDir":               remoteDir,
		"RootArgoAppManifestPath": remoteRootArgoAppManifestPath,
		"NoReboot":                noReboot,
	})
}

func joinInstallScript(remoteDir string, nodeName string, role nodeRole, hasNetwork bool, noReboot bool) string {
	service := "k3s-agent"
	if role == nodeRoleServer {
		service = "k3s"
	}
	return renderScript("join-install.sh.tmpl", map[string]any{
		"RemoteDir":      remoteDir,
		"Role":           string(role),
		"ConfigFile":     "30-k2-" + string(role) + ".yaml",
		"ActivationFile": "99-k2-k3s-" + string(role) + ".yaml",
		"Service":        service,
		"HasNetwork":     hasNetwork,
		"IsServer":       role == nodeRoleServer,
		"NoReboot":       noReboot,
	})
}
