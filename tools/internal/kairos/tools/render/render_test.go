package render

import (
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
)

func TestBootstrapConfigAddsAutomaticServerLabelsAndTaints(t *testing.T) {
	data, err := BootstrapConfig(BootstrapInput{
		Cluster:  clusterconfig.Config{},
		NodeName: "node-01",
		Labels:   []string{"example.com/custom=true"},
		Taints:   []string{"example.com/custom=true:NoSchedule"},
		ImageMetadata: ImageMetadata{
			Target:   "ubuntu-26.04-arm64-qemu-k8s",
			Arch:     "arm64",
			Hardware: "qemu",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"cluster-init: true",
		"node-name: node-01",
		"k2.wyvernzora.io/hardware=qemu",
		"k2.wyvernzora.io/image-target=ubuntu-26.04-arm64-qemu-k8s",
		"k2.wyvernzora.io/arch=arm64",
		"node-role.kubernetes.io/control-plane=true:NoSchedule",
		"example.com/custom=true",
		"example.com/custom=true:NoSchedule",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("bootstrap config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "node-label:\n    - node-role.kubernetes.io/control-plane=true") ||
		strings.Contains(got, "node-label:\n- node-role.kubernetes.io/control-plane=true") {
		t.Fatalf("bootstrap config should not set reserved control-plane node label through kubelet:\n%s", got)
	}
}

func TestBootstrapConfigAcceptsTaintWithoutValue(t *testing.T) {
	data, err := BootstrapConfig(BootstrapInput{
		NodeName: "node-01",
		Taints:   []string{"example.com/dedicated:NoSchedule"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, "example.com/dedicated:NoSchedule") {
		t.Fatalf("bootstrap config missing taint:\n%s", got)
	}
}

func TestBootstrapConfigRejectsConflictingLabels(t *testing.T) {
	_, err := BootstrapConfig(BootstrapInput{
		NodeName:      "node-01",
		Labels:        []string{"k2.wyvernzora.io/hardware=other"},
		ImageMetadata: ImageMetadata{Hardware: "qemu"},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting label") {
		t.Fatalf("error = %v", err)
	}
}

func TestBootstrapConfigRejectsReservedKubeletLabel(t *testing.T) {
	_, err := BootstrapConfig(BootstrapInput{
		NodeName: "node-01",
		Labels:   []string{"node-role.kubernetes.io/control-plane=true"},
	})
	if err == nil || !strings.Contains(err.Error(), "reserved kubelet label") {
		t.Fatalf("error = %v", err)
	}
}

func TestBootstrapConfigAllowsWhitelistedKubeletReservedLabel(t *testing.T) {
	_, err := BootstrapConfig(BootstrapInput{
		NodeName: "node-01",
		Labels:   []string{"kubernetes.io/hostname=node-01"},
	})
	if err != nil {
		t.Fatalf("expected reserved label allowlist to pass: %v", err)
	}
}

func TestClusterConfig(t *testing.T) {
	cfg := clusterconfig.Config{}
	cfg.Kubernetes.API = "10.10.9.1"
	cfg.Kubernetes.DNS = "10.43.0.10"
	cfg.Kubernetes.Domain = "cluster.local"
	cfg.Kubernetes.Subnets.Pods = "10.42.0.0/16"
	cfg.Kubernetes.Subnets.Services = "10.43.0.0/16"
	cfg.AWS.OIDCIssuer.URL = "https://oidc.k2.wyvernzora.io/v3"
	cfg.AWS.OIDCIssuer.JWKSURI = "https://oidc.k2.wyvernzora.io/v3/openid/v1/jwks"

	data, err := ClusterConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"cluster-cidr: 10.42.0.0/16",
		"service-cidr: 10.43.0.0/16",
		"cluster-dns: 10.43.0.10",
		"cluster-domain: cluster.local",
		"- 10.10.9.1",
		"- service-account-issuer=https://oidc.k2.wyvernzora.io/v3",
		"- service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"- service-account-jwks-uri=https://oidc.k2.wyvernzora.io/v3/openid/v1/jwks",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cluster config missing %q:\n%s", want, got)
		}
	}
}

func TestActivationCloudConfigSetsHostnameAndEnablesK3s(t *testing.T) {
	got := string(ActivationCloudConfig("v3-test-01", []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFake operator"}))
	for _, want := range []string{
		"#cloud-config",
		"name: K2 K3s server activation",
		"stages:",
		"initramfs:",
		"name: Set local hostname",
		"hostname: v3-test-01",
		"k3s:",
		"enabled: true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("activation config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "passwd:") {
		t.Fatalf("activation config should not lock the default password before verification:\n%s", got)
	}
	if strings.Contains(got, "users:") {
		t.Fatalf("activation config should not mutate user state:\n%s", got)
	}
}

func TestJoinConfigRendersServerJoinWithControlPlaneTaint(t *testing.T) {
	data, err := JoinConfig(JoinInput{
		NodeName:     "server-02",
		ServerURL:    "https://10.10.9.1:6443",
		Token:        "server-token",
		ControlPlane: true,
		ImageMetadata: ImageMetadata{
			Target:   "ubuntu-26.04-arm64-rpi4cb-k8s",
			Arch:     "arm64",
			Hardware: "rpi4cb",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"server: https://10.10.9.1:6443",
		"token: server-token",
		"node-name: server-02",
		"k2.wyvernzora.io/hardware=rpi4cb",
		"node-role.kubernetes.io/control-plane=true:NoSchedule",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("server join config missing %q:\n%s", want, got)
		}
	}
}

func TestJoinConfigRendersWorkerWithoutControlPlaneTaint(t *testing.T) {
	data, err := JoinConfig(JoinInput{
		NodeName:  "worker-01",
		ServerURL: "https://10.10.9.1:6443",
		Token:     "agent-token",
		Taints:    []string{"example.com/gpu=true:NoSchedule"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"server: https://10.10.9.1:6443",
		"token: agent-token",
		"node-name: worker-01",
		"example.com/gpu=true:NoSchedule",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker join config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "node-role.kubernetes.io/control-plane") {
		t.Fatalf("worker join config unexpectedly has control-plane taint:\n%s", got)
	}
}

func TestAgentActivationCloudConfigEnablesK3sAgent(t *testing.T) {
	got := string(AgentActivationCloudConfig("worker-01", []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFake operator"}))
	for _, want := range []string{
		"#cloud-config",
		"name: K2 K3s worker activation",
		"stages:",
		"initramfs:",
		"name: Set local hostname",
		"hostname: worker-01",
		"k3s-agent:",
		"enabled: true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("agent activation config missing %q:\n%s", want, got)
		}
	}
}
