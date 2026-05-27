package toolcli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
)

func rootArgoAppApplyScript(manifestPath string) string {
	return strings.Join([]string{
		"set -eu",
		"sudo kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=30s >/dev/null",
		fmt.Sprintf("sudo kubectl apply -f %s", shellQuote(manifestPath)),
	}, "\n")
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
