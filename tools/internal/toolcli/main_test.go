package toolcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
	testvm "github.com/wyvernzora/k2/tools/internal/kairos/tools/vm"
)

func TestInstallScriptInstallsBootstrapFilesWithoutLockingDefaultPassword(t *testing.T) {
	got := installScript("/tmp/k2-tools.test", "v3-test-01", true)
	for _, want := range []string{
		"sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml",
		"sudo install -m 0644 \"/tmp/k2-tools.test\"/k2-root-argocd-app.k8s.yaml /var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml",
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
		"sudo test -s /var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml",
		"sudo kubectl -n argocd get application k2 >/dev/null",
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

func TestRootArgoAppApplyScriptWaitsForCRDThenApplies(t *testing.T) {
	got := rootArgoAppApplyScript("/var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml")
	for _, want := range []string{
		"sudo kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=30s >/dev/null",
		"sudo kubectl apply -f '/var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("root Argo app apply script missing %q:\n%s", want, got)
		}
	}
}

func TestWriteBundleIncludesRootArgoAppManifest(t *testing.T) {
	dir := t.TempDir()
	err := writeBundle(dir, bundle{
		ClusterConfig:   []byte("cluster"),
		BootstrapConfig: []byte("bootstrap"),
		Activation:      []byte("activation"),
		AuthorizedKeys:  []byte("keys"),
		Manifests:       []byte("manifests"),
		RootArgoApp:     []byte("root-app"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "k2-root-argocd-app.k8s.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "root-app" {
		t.Fatalf("root app manifest = %q, want root-app", string(got))
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

func TestApplyTestKubeVIPRewritesAPI(t *testing.T) {
	cfg := testClusterConfig()
	applyTestKubeVIP(&cfg, "192.168.64.254")

	if cfg.Kubernetes.API != "192.168.64.254" {
		t.Fatalf("api = %s, want 192.168.64.254", cfg.Kubernetes.API)
	}
}

func TestNodeLabelsForRolePreservesWorkerLabels(t *testing.T) {
	got, err := nodeLabelsForRole(nodeRoleWorker, []string{"example.com/custom=true"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "example.com/custom=true") {
		t.Fatalf("worker labels missing custom label: %v", got)
	}
	if contains(got, "node.longhorn.io/create-default-disk=true") {
		t.Fatalf("Longhorn storage label should be applied after worker join, not during K3s registration: %v", got)
	}
}

func TestNodeLabelsForRoleRejectsLonghornLabelsOnServers(t *testing.T) {
	_, err := nodeLabelsForRole(nodeRoleServer, []string{"node.longhorn.io/create-default-disk=true"})
	if err == nil || !strings.Contains(err.Error(), "k2-tools manages Longhorn") {
		t.Fatalf("error = %v", err)
	}
}

func TestNodeLabelsForRoleRejectsLonghornLabelsOnWorkers(t *testing.T) {
	_, err := nodeLabelsForRole(nodeRoleWorker, []string{"node.longhorn.io/create-default-disk=true"})
	if err == nil || !strings.Contains(err.Error(), "k2-tools manages Longhorn") {
		t.Fatalf("error = %v", err)
	}
}

func TestRejectLonghornNodeLabelsCatchesPrefixWithoutValue(t *testing.T) {
	err := rejectLonghornNodeLabels("bootstrap", []string{"node.longhorn.io/create-default-disk"})
	if err == nil || !strings.Contains(err.Error(), "k2-tools manages Longhorn") {
		t.Fatalf("error = %v", err)
	}
}

func TestMarkLonghornStorageNodeRetriesUntilNodeExists(t *testing.T) {
	temporary := errors.New("not found")
	marker := &fakeLonghornMarker{annotateErrs: []error{temporary, temporary, nil}}
	var out bytes.Buffer

	err := markLonghornStorageNodeWithRetry(context.Background(), marker, "v3-worker-01", &out, time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("markLonghornStorageNodeWithRetry: %v", err)
	}
	if marker.annotateCalls != 3 {
		t.Fatalf("annotate calls = %d, want 3", marker.annotateCalls)
	}
	if marker.labelCalls != 1 {
		t.Fatalf("label calls = %d, want 1", marker.labelCalls)
	}
	if !strings.Contains(out.String(), "waiting for Kubernetes node v3-worker-01") {
		t.Fatalf("expected retry output, got:\n%s", out.String())
	}
}

func TestApplyLonghornStorageNodeMarkAnnotatesBeforeLabel(t *testing.T) {
	marker := &fakeLonghornMarker{}
	if err := applyLonghornStorageNodeMark(context.Background(), marker, "v3-worker-01"); err != nil {
		t.Fatalf("applyLonghornStorageNodeMark: %v", err)
	}
	if got := strings.Join(marker.calls, ","); got != "annotate,label" {
		t.Fatalf("call order = %s, want annotate,label", got)
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

type fakeLonghornMarker struct {
	calls         []string
	annotateErrs  []error
	labelErrs     []error
	annotateCalls int
	labelCalls    int
}

func (m *fakeLonghornMarker) AnnotateNode(ctx context.Context, node string, keyValue string) error {
	m.calls = append(m.calls, "annotate")
	m.annotateCalls++
	if len(m.annotateErrs) == 0 {
		return nil
	}
	err := m.annotateErrs[0]
	m.annotateErrs = m.annotateErrs[1:]
	return err
}

func (m *fakeLonghornMarker) LabelNode(ctx context.Context, node string, keyValue string) error {
	m.calls = append(m.calls, "label")
	m.labelCalls++
	if len(m.labelErrs) == 0 {
		return nil
	}
	err := m.labelErrs[0]
	m.labelErrs = m.labelErrs[1:]
	return err
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func testClusterConfig() clusterconfig.Config {
	cfg := clusterconfig.Config{ID: "v3"}
	cfg.Kubernetes.API = "10.10.9.1"
	cfg.Kubernetes.DNS = "10.43.0.10"
	cfg.Kubernetes.Domain = "cluster.local"
	cfg.Kubernetes.Subnets.Pods = "10.42.0.0/16"
	cfg.Kubernetes.Subnets.Services = "10.43.0.0/16"
	return cfg
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
