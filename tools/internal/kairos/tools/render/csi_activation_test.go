package render

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// CONTRACT: the csi user must be recreated by a boot stage. Kairos /etc is
// an ephemeral overlay — a provision-time useradd (and /etc/sudoers.d
// content) vanishes on the first reboot, breaking democratic-csi's SSH.
func TestCSIUserActivationCloudConfigRecreatesUserEveryBoot(t *testing.T) {
	got := string(CSIUserActivationCloudConfig("ssh-ed25519 AAAA test", "csi ALL=(ALL) NOPASSWD:ALL\n"))
	if !strings.HasPrefix(got, "#cloud-config\n") {
		t.Fatalf("missing #cloud-config header:\n%s", got)
	}
	var tree map[string]any
	if err := yaml.Unmarshal([]byte(got), &tree); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"initramfs:",
		"csi:",
		"ssh-ed25519 AAAA test",
		"/etc/sudoers.d/99-csi",
		"csi ALL=(ALL) NOPASSWD:ALL",
		"chown -R csi:csi /home/csi",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("csi activation missing %q:\n%s", want, got)
		}
	}
}
