package toolcli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestE2EShippedScenarioPlans(t *testing.T) {
	repoRoot := testRepoRoot(t)
	tests := []struct {
		name string
		want []string
	}{
		{
			name: "storage-pvc",
			want: []string{
				"Preflight|preflight|Check local artifacts and tools",
				"Preflight|operatorKey|Generate e2e operator key",
				"Create VMs|vm|Create/start VM e2e-k2e2e-storage",
				"Create VMs|vm|Create/start VM e2e-k2e2e-server",
				"Create VMs|vm|Create/start VM e2e-k2e2e-worker-1",
				"Create VMs|vmWait|Wait for VM e2e-k2e2e-storage",
				"Create VMs|vmWait|Wait for VM e2e-k2e2e-server",
				"Create VMs|vmWait|Wait for VM e2e-k2e2e-worker-1",
				"Provision|storage|Provision storage appliance e2e-k2e2e-storage",
				"Provision|bootstrap|Provision bootstrap server e2e-k2e2e-server",
				"Provision|worker|Provision worker e2e-k2e2e-worker-1",
				"Steps|rebootVM|Reboot storage and wait for active boot",
				"Steps|nodePrepISCSI|Prepare node iSCSI initiators",
				"Steps|helmInstall|Install zfs-iscsi",
				"Checks|pvcLifecycle|PVC lifecycle",
				"Checks|zfsConsistency|ZFS consistency",
				"Checks|deleteHygiene|Delete hygiene",
				"Teardown|teardown|Remove e2e resources",
			},
		},
		{
			name: "k8s-wireup",
			want: []string{
				"Preflight|preflight|Check local artifacts and tools",
				"Preflight|operatorKey|Generate e2e operator key",
				"Create VMs|vm|Create/start VM e2e-k2e2e-server",
				"Create VMs|vm|Create/start VM e2e-k2e2e-worker-1",
				"Create VMs|vmWait|Wait for VM e2e-k2e2e-server",
				"Create VMs|vmWait|Wait for VM e2e-k2e2e-worker-1",
				"Provision|bootstrap|Provision bootstrap server e2e-k2e2e-server",
				"Provision|worker|Provision worker e2e-k2e2e-worker-1",
				"Checks|nodesReady|Nodes Ready",
				"Teardown|teardown|Remove e2e resources",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario, err := loadE2EScenario(repoRoot, tt.name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := e2eExecutionPlan(scenario, defaultE2ETestOptions())
			if err != nil {
				t.Fatal(err)
			}
			gotStrings := planStrings(got)
			if !reflect.DeepEqual(gotStrings, tt.want) {
				t.Fatalf("plan mismatch\n got: %#v\nwant: %#v", gotStrings, tt.want)
			}
		})
	}
}

func TestE2EScenarioStrictLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "unknown-top",
			body:    baseScenarioYAML("unknown-top", "bogus: true\nsteps: []\nchecks: []\n"),
			wantErr: "field bogus not found",
		},
		{
			name:    "unknown-step",
			body:    baseScenarioYAML("unknown-step", "steps:\n  - shell: {}\nchecks: []\n"),
			wantErr: "unknown step type \"shell\"",
		},
		{
			name:    "unknown-values",
			body:    baseScenarioYAML("unknown-values", "steps:\n  - helmInstall:\n      release: zfs-iscsi\n      chart: democratic-csi/democratic-csi\n      repo: https://example.invalid/charts/\n      values: nope\nchecks: []\n"),
			wantErr: "unknown values generator \"nope\"",
		},
		{
			name:    "bad-vm-ref",
			body:    baseScenarioYAML("bad-vm-ref", "provision:\n  - worker:\n      vm: missing\nsteps: []\nchecks: []\n"),
			wantErr: "references undeclared vm \"missing\"",
		},
		{
			name:    "duplicate-vm",
			body:    "name: duplicate-vm\ndescription: duplicate vm test\nvms:\n  - name: server\n    preset: qemu-vmnet\n  - name: server\n    preset: qemu-vmnet\nprovision: []\nsteps: []\nchecks: []\n",
			wantErr: "duplicate vm name \"server\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeScenario(t, tt.name, tt.body)
			_, err := loadE2EScenarioPath(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestE2EStorageAliasPlanMatchesRunStoragePVC(t *testing.T) {
	repoRoot := testRepoRoot(t)
	runScenario, err := loadE2EScenario(repoRoot, storagePVCScenarioName)
	if err != nil {
		t.Fatal(err)
	}
	runPlan, err := e2eExecutionPlan(runScenario, defaultE2ETestOptions())
	if err != nil {
		t.Fatal(err)
	}

	aliasScenario, err := loadE2EScenario(repoRoot, storagePVCScenarioName)
	if err != nil {
		t.Fatal(err)
	}
	storageCmd := (e2eStorageCmd{}).defaults()
	applyStorageAliasOverrides(aliasScenario, storageCmd)
	aliasPlan, err := e2eExecutionPlan(aliasScenario, storageCmd.options())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(planStrings(aliasPlan), planStrings(runPlan)) {
		t.Fatalf("alias plan = %#v, run plan = %#v", planStrings(aliasPlan), planStrings(runPlan))
	}
}

func defaultE2ETestOptions() e2eRunOptions {
	return e2eRunOptions{
		scenarioName:  storagePVCScenarioName,
		clusterTarget: "v3",
		clusterName:   "k2e2e",
	}
}

func planStrings(plan []e2ePlanStep) []string {
	out := make([]string, len(plan))
	for i, step := range plan {
		out[i] = step.Phase + "|" + step.Kind + "|" + step.Label
	}
	return out
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeScenario(t *testing.T, name string, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseScenarioYAML(name string, tail string) string {
	return "name: " + name + "\ndescription: strict load test\nvms:\n  - name: server\n    preset: qemu-vmnet\n" + tail
}
