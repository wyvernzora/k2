package toolcli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestE2EShippedScenarioWorkflowNames(t *testing.T) {
	repoRoot := testRepoRoot(t)
	tests := []struct {
		name string
		want []string
	}{
		{
			name: "storage-pvc",
			want: []string{
				"sudo launch QEMU with vmnet networking",
				"section Preflight",
				"keyvalues",
				"shell Check local artifacts and tools",
				"task Generate e2e operator key",
				"section Create VMs",
				"shell Create/start VM e2e-k2e2e-storage",
				"shell Create/start VM e2e-k2e2e-server",
				"shell Create/start VM e2e-k2e2e-worker-1",
				"shell Wait for VM e2e-k2e2e-storage",
				"shell Wait for VM e2e-k2e2e-server",
				"shell Wait for VM e2e-k2e2e-worker-1",
				"section Provision",
				"run Provision storage appliance e2e-k2e2e-storage",
				"run Provision bootstrap server e2e-k2e2e-server",
				"run Provision worker e2e-k2e2e-worker-1",
				"section Steps",
				"shell Reboot storage and wait for active boot",
				"shell Prepare node iSCSI initiators",
				"shell Install zfs-iscsi",
				"section Checks",
				"shell PVC lifecycle",
				"shell ZFS consistency",
				"shell Storage metrics",
				"shell Delete hygiene",
				"section Teardown",
				"shell Remove e2e resources",
			},
		},
		{
			name: "k8s-wireup",
			want: []string{
				"sudo launch QEMU with vmnet networking",
				"section Preflight",
				"keyvalues",
				"shell Check local artifacts and tools",
				"task Generate e2e operator key",
				"section Create VMs",
				"shell Create/start VM e2e-k2e2e-server",
				"shell Create/start VM e2e-k2e2e-worker-1",
				"shell Wait for VM e2e-k2e2e-server",
				"shell Wait for VM e2e-k2e2e-worker-1",
				"section Provision",
				"run Provision bootstrap server e2e-k2e2e-server",
				"run Provision worker e2e-k2e2e-worker-1",
				"section Steps",
				"section Checks",
				"shell Nodes Ready",
				"section Teardown",
				"shell Remove e2e resources",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shippedScenarioNames(t, repoRoot, tt.name, defaultE2ETestOptions())
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("workflow names mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestE2EStorageAliasWorkflowMatchesRunStoragePVC(t *testing.T) {
	repoRoot := testRepoRoot(t)
	runNames := shippedScenarioNames(t, repoRoot, storagePVCScenarioName, defaultE2ETestOptions())

	aliasScenario, err := loadE2EScenario(repoRoot, storagePVCScenarioName)
	if err != nil {
		t.Fatal(err)
	}
	aliasCmd := e2eStorageCmd{Workers: 1, ClusterTarget: "v3", ClusterName: "k2e2e", Namespace: "e2e-storage", PVCSize: "1Gi"}
	applyStorageAliasOverrides(aliasScenario, aliasCmd)
	aliasNames := scenarioNames(t, repoRoot, aliasScenario, aliasCmd.options())

	if !reflect.DeepEqual(aliasNames, runNames) {
		t.Fatalf("alias names = %#v, run names = %#v", aliasNames, runNames)
	}
}

func shippedScenarioNames(t *testing.T, repoRoot string, name string, opts e2eRunOptions) []string {
	t.Helper()
	scenario, err := loadE2EScenario(repoRoot, name)
	if err != nil {
		t.Fatal(err)
	}
	return scenarioNames(t, repoRoot, scenario, opts)
}

// scenarioNames builds the REAL workflow (nothing executes: step closures
// are only constructed) and returns its step names.
func scenarioNames(t *testing.T, repoRoot string, scenario *e2eScenario, opts e2eRunOptions) []string {
	t.Helper()
	applyE2ERunOverrides(scenario, opts)
	if err := validateE2EScenario(scenario.Name, scenario); err != nil {
		t.Fatal(err)
	}
	state := newE2EScenarioState(scenario, opts, t.TempDir())
	rcx := &runContext{repoRoot: repoRoot}
	return buildE2EWorkflow(rcx, state, opts, scenario).Names()
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
			name:    "bad-storage-metrics-vm-ref",
			body:    baseScenarioYAML("bad-storage-metrics-vm-ref", "steps: []\nchecks:\n  - storageMetrics:\n      vm: missing\n"),
			wantErr: "storageMetrics references undeclared vm \"missing\"",
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

func TestE2EStorageMetricsCheckLabel(t *testing.T) {
	entry := e2eScenarioCheck{Type: "storageMetrics", StorageMetrics: e2eVMCheck{VM: "storage"}}
	if got := e2eCheckLabel(entry); got != "Storage metrics" {
		t.Fatalf("label = %q, want %q", got, "Storage metrics")
	}
}

func defaultE2ETestOptions() e2eRunOptions {
	return e2eRunOptions{
		clusterTarget: "v3",
		clusterName:   "k2e2e",
	}
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
