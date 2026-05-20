package main

import (
	"strings"
	"testing"
)

func TestInstallScriptSetsHostnameAndLocksDefaultPassword(t *testing.T) {
	got := installScript("/tmp/k2-provision.test", "v3-test-01", true)
	for _, want := range []string{
		"printf '127.0.1.1 v3-test-01 kairos\\n' | sudo tee -a /etc/hosts >/dev/null",
		"sudo hostnamectl set-hostname \"v3-test-01\"",
		"sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip",
		"sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip",
		"sudo install -m 0644 \"/tmp/k2-provision.test\"/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml",
		"sudo mv /oem/90_custom.yaml /oem/90_custom.yaml.k2-disabled",
		"sudo install -o kairos -g kairos -m 0600 \"/tmp/k2-provision.test\"/operator_authorized_keys /home/kairos/.ssh/authorized_keys",
		"sudo passwd -l kairos",
		"sudo systemctl enable k3s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install script missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "sudo reboot") {
		t.Fatalf("install script unexpectedly reboots with noReboot=true:\n%s", got)
	}
}

func TestJoinInstallScriptForServerActivatesInvariantConfig(t *testing.T) {
	got := joinInstallScript("/tmp/k2-provision.test", "v3-server-02", nodeRoleServer, true)
	for _, want := range []string{
		"sudo hostnamectl set-hostname \"v3-server-02\"",
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
		"sudo hostnamectl set-hostname \"v3-worker-01\"",
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
