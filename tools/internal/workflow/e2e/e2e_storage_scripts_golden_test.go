package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateScriptGoldens = flag.Bool("update-script-goldens", false, "rewrite generated-script goldens")

// These run as root on nodes and on the appliance during an E2E run, so pin
// their exact bytes rather than trusting a Contains-style assertion.
func TestGeneratedScriptsMatchGoldens(t *testing.T) {
	creds := storageCredentials{
		Pool:                               "tank",
		DatasetParentName:                  "tank/csi/k2",
		DetachedSnapshotsDatasetParentName: "tank/csi/k2-snapshots",
		IQNBase:                            "iqn.2026-07.io.wyvernzora.k2:storage",
	}

	cases := []struct {
		golden string
		script string
	}{
		{"node-iscsi-prep.sh", e2eNodeISCSIPrepScript()},
		{"storage-consistency.sh", e2eStorageConsistencyScript(creds, "pvc-abc123", 1073741824)},
		{"storage-cleanup-poll.sh", e2eStorageCleanupPollScript(creds, "pvc-abc123")},
		{"node-no-iscsi-session.sh", e2eNodeNoISCSISessionScript(creds.IQNBase)},
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
