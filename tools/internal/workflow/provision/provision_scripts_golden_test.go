package provision

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateScriptGoldens = flag.Bool("update-script-goldens", false, "rewrite generated-script goldens")

// Every remote script this package generates, rendered across the branches that
// change their output. These run as root on a node during provisioning, so a
// stray quote or a dropped line is expensive and invisible in a Contains-style
// assertion — the goldens pin the exact bytes.
func TestGeneratedScriptsMatchGoldens(t *testing.T) {
	vdevs := []storageVDev{
		{Topology: "mirror", Devices: []string{"/dev/disk/by-id/ata-a", "/dev/disk/by-id/ata-b"}},
	}

	cases := []struct {
		golden string
		script string
	}{
		{"bootstrap-verify.sh", bootstrapVerificationScript("k2-node")},
		{"join-verify-server.sh", joinVerificationScript("k2-node", nodeRoleServer, true)},
		{"join-verify-worker.sh", joinVerificationScript("k2-node", nodeRoleWorker, false)},
		{"install.sh", installScript("/tmp/k2 remote", "k2-node", false)},
		{"install-noreboot.sh", installScript("/tmp/k2-remote", "k2-node", true)},
		{"join-install-server.sh", joinInstallScript("/tmp/k2-remote", "k2-node", nodeRoleServer, true, false)},
		{"join-install-worker.sh", joinInstallScript("/tmp/k2-remote", "k2-node", nodeRoleWorker, false, true)},
		{"storage-install-full.sh", storageInstallScript("node name", true, true)},
		{"storage-install-minimal.sh", storageInstallScript("k2-storage", false, false)},
		{"storage-pool-import-only.sh", storagePoolScript(storagePoolScriptInput{
			Pool: "tank", ClusterName: "k2", Compatibility: "openzfs-2.2-linux",
		})},
		{"storage-pool-create.sh", storagePoolScript(storagePoolScriptInput{
			Pool: "tank", ClusterName: "k2", Compatibility: "openzfs-2.2-linux",
			VDevs: vdevs, CreateAllowed: true,
		})},
		{"storage-pool-create-force-wipe.sh", storagePoolScript(storagePoolScriptInput{
			Pool: "tank", ClusterName: "k2", Compatibility: "openzfs-2.2-linux",
			VDevs: vdevs, CreateAllowed: true, ForceWipe: true,
		})},
	}

	for _, tt := range cases {
		t.Run(tt.golden, func(t *testing.T) {
			path := filepath.Join("testdata", "scripts", tt.golden)
			if *updateScriptGoldens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.script), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.script != string(want) {
				t.Fatalf("script %s drifted\ngot:\n%s\nwant:\n%s", tt.golden, tt.script, string(want))
			}
		})
	}
}
