package main

import (
	"strings"
	"testing"
)

func TestInstallScriptInstallsBootstrapFilesWithoutLockingDefaultPassword(t *testing.T) {
	got := installScript("/tmp/k2-provision.test", "v3-test-01", true)
	for _, want := range []string{
		"sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip",
		"sudo install -m 0644 \"/tmp/k2-provision.test\"/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml",
		"sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh",
		"sudo install -o kairos -g kairos -m 0600 \"/tmp/k2-provision.test\"/operator_authorized_keys /home/kairos/.ssh/authorized_keys",
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
	got := joinInstallScript("/tmp/k2-provision.test", "v3-server-02", nodeRoleServer, true)
	for _, want := range []string{
		"sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml",
		"sudo install -m 0644 \"/tmp/k2-provision.test\"/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml",
		"sudo install -m 0600 \"/tmp/k2-provision.test\"/30-k2-server.yaml /etc/rancher/k3s/config.yaml.d/30-k2-server.yaml",
		"sudo install -m 0644 \"/tmp/k2-provision.test\"/99-k2-k3s-server.yaml /oem/99-k2-k3s-server.yaml",
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
	got := joinInstallScript("/tmp/k2-provision.test", "v3-worker-01", nodeRoleWorker, true)
	for _, want := range []string{
		"sudo install -m 0600 \"/tmp/k2-provision.test\"/30-k2-worker.yaml /etc/rancher/k3s/config.yaml.d/30-k2-worker.yaml",
		"sudo install -m 0644 \"/tmp/k2-provision.test\"/99-k2-k3s-worker.yaml /oem/99-k2-k3s-worker.yaml",
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
