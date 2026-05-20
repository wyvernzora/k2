package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/kairos/tools/internal/clusterconfig"
	testvm "github.com/wyvernzora/k2/kairos/tools/internal/vm"
)

func TestInstallScriptInstallsBootstrapFilesWithoutLockingDefaultPassword(t *testing.T) {
	got := installScript("/tmp/k2-tools.test", "v3-test-01", true)
	for _, want := range []string{
		"sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml",
		"sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh",
		"sudo install -o kairos -g kairos -m 0600 \"/tmp/k2-tools.test\"/operator_authorized_keys /home/kairos/.ssh/authorized_keys",
		"sudo systemctl enable k3s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install script missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "sudo reboot") {
		t.Fatalf("install script unexpectedly reboots with noReboot=true:\n%s", got)
	}
	if strings.Contains(got, "sudo passwd -l kairos") {
		t.Fatalf("install script locks default password before post-reboot verification:\n%s", got)
	}
	if strings.Contains(got, "90_custom.yaml.k2-disabled") {
		t.Fatalf("install script disables default credentials before post-reboot verification:\n%s", got)
	}
	if strings.Contains(got, "sudo chmod 0700 /home/kairos/.ssh") {
		t.Fatalf("install script chmods a potentially root-owned SSH directory:\n%s", got)
	}
	for _, unwanted := range []string{
		"hostnamectl set-hostname",
		"/etc/hosts",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("install script mutates host identity outside cloud-init with %q:\n%s", unwanted, got)
		}
	}
}

func TestJoinInstallScriptForServerActivatesInvariantConfig(t *testing.T) {
	got := joinInstallScript("/tmp/k2-tools.test", "v3-server-02", nodeRoleServer, true)
	for _, want := range []string{
		"sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml",
		"sudo install -m 0600 \"/tmp/k2-tools.test\"/30-k2-server.yaml /etc/rancher/k3s/config.yaml.d/30-k2-server.yaml",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/99-k2-k3s-server.yaml /oem/99-k2-k3s-server.yaml",
		"sudo systemctl enable k3s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("server install script missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "sudo systemctl enable k3s-agent") {
		t.Fatalf("server install script unexpectedly enables k3s-agent:\n%s", got)
	}
}

func TestJoinInstallScriptForWorkerUsesAgentService(t *testing.T) {
	got := joinInstallScript("/tmp/k2-tools.test", "v3-worker-01", nodeRoleWorker, true)
	for _, want := range []string{
		"sudo install -m 0600 \"/tmp/k2-tools.test\"/30-k2-worker.yaml /etc/rancher/k3s/config.yaml.d/30-k2-worker.yaml",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/99-k2-k3s-worker.yaml /oem/99-k2-k3s-worker.yaml",
		"sudo systemctl enable k3s-agent",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker install script missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"10-k2-invariant.yaml",
		"20-k2-cluster.yaml",
		"server/manifests/traefik.yaml.skip",
		"sudo systemctl enable k3s\n",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("worker install script unexpectedly contains %q:\n%s", unwanted, got)
		}
	}
}

func TestBootstrapVerificationScriptChecksExpectedState(t *testing.T) {
	got := bootstrapVerificationScript("v3-test-01")
	for _, want := range []string{
		"test \"$(hostname)\" = 'v3-test-01'",
		"sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml",
		"sudo test -s /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml",
		"sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml",
		"sudo test -s /oem/99-k2-k3s-bootstrap.yaml",
		"systemctl is-enabled --quiet k3s",
		"systemctl is-active --quiet k3s",
		"sudo test -s /var/lib/rancher/k3s/server/token",
		"sudo test -f /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("bootstrap verification script missing %q:\n%s", want, got)
		}
	}
}

func TestJoinVerificationScriptChecksRoleSpecificState(t *testing.T) {
	server := joinVerificationScript("v3-server-02", nodeRoleServer)
	for _, want := range []string{
		"sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-server.yaml",
		"sudo test -s /oem/99-k2-k3s-server.yaml",
		"sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml",
		"systemctl is-enabled --quiet k3s",
		"systemctl is-active --quiet k3s",
	} {
		if !strings.Contains(server, want) {
			t.Fatalf("server verification script missing %q:\n%s", want, server)
		}
	}

	worker := joinVerificationScript("v3-worker-01", nodeRoleWorker)
	for _, want := range []string{
		"sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-worker.yaml",
		"sudo test -s /oem/99-k2-k3s-worker.yaml",
		"sudo test ! -e /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml",
		"systemctl is-enabled --quiet k3s-agent",
		"systemctl is-active --quiet k3s-agent",
	} {
		if !strings.Contains(worker, want) {
			t.Fatalf("worker verification script missing %q:\n%s", want, worker)
		}
	}
	if strings.Contains(worker, "systemctl is-active --quiet k3s\n") {
		t.Fatalf("worker verification script unexpectedly checks server service:\n%s", worker)
	}
}

func TestTestKubeVIPUsesLastUsableAddressInNodeCIDR(t *testing.T) {
	got, err := testKubeVIP("192.168.64.2", 24)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.64.254" {
		t.Fatalf("vip = %s, want 192.168.64.254", got)
	}

	got, err = testKubeVIP("192.168.64.254", 24)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.64.253" {
		t.Fatalf("vip = %s, want 192.168.64.253", got)
	}
}

func TestApplyTestKubeVIPPatchesVIPAndTLSSAN(t *testing.T) {
	cfg := testClusterConfig()
	applyTestKubeVIP(&cfg, "192.168.64.254")

	if cfg.Kubernetes.API.VIP != "192.168.64.254" {
		t.Fatalf("vip = %s, want 192.168.64.254", cfg.Kubernetes.API.VIP)
	}
	if cfg.Kubernetes.API.DNSName != "" {
		t.Fatalf("dns name = %s, want empty for VM test endpoint", cfg.Kubernetes.API.DNSName)
	}
	if !containsString(cfg.Kubernetes.API.TLSSans, "192.168.64.254") {
		t.Fatalf("tls sans missing test VIP: %#v", cfg.Kubernetes.API.TLSSans)
	}
}

func TestApplyProvisionTestVMDefaultsClusterNodeAndSSH(t *testing.T) {
	root := t.TempDir()
	writeTestVMMetadata(t, root, testvm.Metadata{
		Backend: "qemu",
		ID:      "v3a",
		VMDir:   filepath.Join(root, ".testvm", "vm-v3a"),
		SSHPort: 2222,
	})

	clusterName := ""
	nodeName := ""
	host := ""
	sshPort := 22
	target, err := applyProvisionTestVM(root, "v3", &clusterName, &nodeName, &host, &sshPort, "v3a")
	if err != nil {
		t.Fatal(err)
	}

	if !target.Enabled {
		t.Fatal("test VM target was not enabled")
	}
	if clusterName != "v3-vmtest" {
		t.Fatalf("clusterName = %s, want v3-vmtest", clusterName)
	}
	if nodeName != "v3a" {
		t.Fatalf("nodeName = %s, want v3a", nodeName)
	}
	if host != "127.0.0.1" {
		t.Fatalf("host = %s, want 127.0.0.1", host)
	}
	if sshPort != 2222 {
		t.Fatalf("sshPort = %d, want 2222", sshPort)
	}
}

func TestApplyProvisionTestVMKeepsExplicitNodeName(t *testing.T) {
	root := t.TempDir()
	writeTestVMMetadata(t, root, testvm.Metadata{
		Backend: "qemu",
		ID:      "v3a",
		VMDir:   filepath.Join(root, ".testvm", "vm-v3a"),
		SSHPort: 2222,
	})

	clusterName := ""
	nodeName := "custom-node"
	host := ""
	sshPort := 22
	_, err := applyProvisionTestVM(root, "v3", &clusterName, &nodeName, &host, &sshPort, "v3a")
	if err != nil {
		t.Fatal(err)
	}
	if nodeName != "custom-node" {
		t.Fatalf("nodeName = %s, want custom-node", nodeName)
	}
}

func testClusterConfig() clusterconfig.Config {
	cfg := clusterconfig.Config{ID: "v3"}
	cfg.Kubernetes.API.VIP = "10.10.9.1"
	cfg.Kubernetes.API.DNSName = "k8s-api.wyvernzora.io"
	cfg.Kubernetes.API.TLSSans = []string{"10.10.9.1", "k8s-api.wyvernzora.io"}
	return cfg
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func writeTestVMMetadata(t *testing.T, root string, meta testvm.Metadata) {
	t.Helper()
	if err := os.MkdirAll(meta.VMDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meta.VMDir, "vm.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
