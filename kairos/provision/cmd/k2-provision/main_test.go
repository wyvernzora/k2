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
