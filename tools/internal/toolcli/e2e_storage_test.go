package toolcli

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSanitizeE2EName(t *testing.T) {
	// Kong default tags own the flag defaults now; only the name
	// sanitizer retains logic worth pinning.
	if got := sanitizeE2EName("K2 E2E"); got != "k2-e2e" {
		t.Fatalf("sanitizeE2EName = %q", got)
	}
	if got := sanitizeE2EName("---"); got != "k2e2e" {
		t.Fatalf("sanitizeE2EName empty-fallback = %q", got)
	}
}

func TestDemocraticCSIValuesYAML(t *testing.T) {
	creds := storageCredentials{
		Portal:                             "192.168.64.10:3260",
		IQNBase:                            "iqn.test:k2",
		Pool:                               "tank",
		DatasetParentName:                  "tank/csi/k2e2e",
		DetachedSnapshotsDatasetParentName: "tank/csi/k2e2e-snapshots",
		SSHHost:                            "192.168.64.10",
		SSHPort:                            22,
		SSHUser:                            "csi",
		CSIPrivateKey:                      "PRIVATE\nKEY\n",
		CHAPUsername:                       "k2-k2e2e",
		CHAPPassword:                       "chap-secret",
	}
	got, err := democraticCSIValuesYAML(creds)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		"name: org.democratic-csi.iscsi",
		"name: zfs-iscsi",
		"node-db.node.session.auth.authmethod: CHAP",
		"node-db.node.session.auth.username: k2-k2e2e",
		"node-db.node.session.auth.password: chap-secret",
		"privateKey: |",
		"PRIVATE\n",
		"KEY",
		"targetPortal: 192.168.64.10:3260",
		"datasetParentName: tank/csi/k2e2e",
		"detachedSnapshotsDatasetParentName: tank/csi/k2e2e-snapshots",
		"basename: iqn.test:k2",
		"iscsiDirHostPath: /etc/iscsi",
		"iscsiDirHostPathType: DirectoryOrCreate",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("values YAML missing %q:\n%s", want, text)
		}
	}

	// Flat substring checks cannot catch a value rendered at the wrong
	// nesting level, and democratic-csi silently ignores misplaced config
	// keys. Walk the exact paths the driver reads.
	var tree map[string]any
	if err := yaml.Unmarshal(got, &tree); err != nil {
		t.Fatal(err)
	}
	walk := func(path ...string) any {
		node := any(tree)
		for _, key := range path {
			m, ok := node.(map[string]any)
			if !ok {
				t.Fatalf("values YAML: %v is not a map on the way to %v", node, path)
			}
			node = m[key]
			if node == nil {
				t.Fatalf("values YAML missing %v", path)
			}
		}
		return node
	}
	if got := walk("driver", "config", "iscsi", "targetPortal"); got != "192.168.64.10:3260" {
		t.Fatalf("driver.config.iscsi.targetPortal = %v", got)
	}
	if got := walk("driver", "config", "iscsi", "shareStrategyTargetCli", "block", "attributes", "emulate_tpu"); got != 1 {
		t.Fatalf("shareStrategyTargetCli.block.attributes.emulate_tpu = %v", got)
	}
	if got := walk("driver", "config", "sshConnection", "host"); got != "192.168.64.10" {
		t.Fatalf("driver.config.sshConnection.host = %v", got)
	}
}

func TestE2ENodePrepScriptRegeneratesInitiator(t *testing.T) {
	got := e2eNodeISCSIPrepScript()
	for _, want := range []string{
		"new_iqn=\"$(iscsi-iname)\"",
		"/etc/iscsi/initiatorname.iscsi",
		"sudo systemctl enable --now iscsid",
		"sudo systemctl restart iscsid",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("node prep script missing %q:\n%s", want, got)
		}
	}
}

func TestE2EStorageConsistencyScriptChecksZFSAndTarget(t *testing.T) {
	creds := storageCredentials{
		Pool:              "tank",
		DatasetParentName: "tank/csi/k2e2e",
		IQNBase:           "iqn.test:k2",
	}
	got := e2eStorageConsistencyScript(creds, "pvc-123", 1073741824)
	for _, want := range []string{
		"sudo zfs list -H -o name -r 'tank/csi/k2e2e'",
		"grep 'pvc-123'",
		"sudo zfs get -Hp -o value volsize \"$zvol\"",
		"test \"$volsize\" -ge 1073741824",
		"sudo zfs get -H -o value keystatus 'tank'",
		"sudo zfs get -H -o value encryption \"$zvol\"",
		"sudo targetcli ls /iscsi | grep 'iqn.test:k2'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("consistency script missing %q:\n%s", want, got)
		}
	}
}

func TestE2EAcceptanceManifestAndIO(t *testing.T) {
	manifest, err := e2eAcceptanceManifest("e2e-storage", "2Gi", "zfs-iscsi")
	if err != nil {
		t.Fatal(err)
	}
	text := string(manifest)
	for _, want := range []string{
		"kind: PersistentVolumeClaim",
		"storageClassName: zfs-iscsi",
		"storage: 2Gi",
		"kind: Pod",
		"claimName: e2e-storage-pvc",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("acceptance manifest missing %q:\n%s", want, text)
		}
	}
	ioScript := e2eIOCheckScript("abc")
	if !strings.Contains(ioScript, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad") {
		t.Fatalf("io check script missing sha256:\n%s", ioScript)
	}
}

func TestParseSimpleQuantityBytes(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{value: "1Gi", want: 1073741824},
		{value: "2Mi", want: 2097152},
		{value: "3G", want: 3000000000},
		{value: "4096", want: 4096},
	}
	for _, tt := range tests {
		got, err := parseSimpleQuantityBytes(tt.value)
		if err != nil {
			t.Fatalf("parseSimpleQuantityBytes(%q): %v", tt.value, err)
		}
		if got != tt.want {
			t.Fatalf("parseSimpleQuantityBytes(%q) = %d, want %d", tt.value, got, tt.want)
		}
	}
	if _, err := parseSimpleQuantityBytes("1.5Gi"); err == nil {
		t.Fatal("parseSimpleQuantityBytes accepted decimal quantity")
	}
}
