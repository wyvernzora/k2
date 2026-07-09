package toolcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/remote"
	testvm "github.com/wyvernzora/k2/tools/internal/kairos/tools/vm"
	"github.com/wyvernzora/k2/tools/internal/ui"
	"gopkg.in/yaml.v3"
)

const storagePVCScenarioName = "storage-pvc"

var (
	e2eProvisionTypes     = []string{"bootstrap", "storage", "worker"}
	e2eStepTypes          = []string{"helmInstall", "nodePrepISCSI"}
	e2eCheckTypes         = []string{"deleteHygiene", "nodesReady", "pvcLifecycle", "zfsConsistency"}
	e2eValueGeneratorKeys = []string{"storageDriverValues"}
)

type e2eListCmd struct{}

type e2eRunCmd struct {
	Scenario           string `arg:"" help:"Scenario name from kairos/tools/e2e-scenarios."`
	Keep               bool   `name:"keep" help:"Preserve VMs and scratch state on exit or failure."`
	ClusterTarget      string `name:"cluster-target" default:"v3" help:"Cluster config/deploy target."`
	ClusterName        string `name:"cluster-name" default:"k2e2e" help:"Local e2e cluster name."`
	Namespace          string `name:"namespace" help:"Override PVC lifecycle namespace."`
	SkipTeardownOnFail bool   `name:"skip-teardown-on-fail" help:"Preserve VMs and scratch state only when the harness fails."`
}

type e2eRunOptions struct {
	scenarioName       string
	keep               bool
	clusterTarget      string
	clusterName        string
	namespace          string
	pvcSize            string
	chartVersion       string
	skipTeardownOnFail bool
}

type e2eScenario struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	VMs         []e2eScenarioVM        `yaml:"vms"`
	Provision   []e2eProvisionEntry    `yaml:"provision"`
	Steps       []e2eScenarioStepEntry `yaml:"steps"`
	Checks      []e2eScenarioCheck     `yaml:"checks"`
}

type e2eScenarioVM struct {
	Name      string                `yaml:"name"`
	Preset    string                `yaml:"preset"`
	Overrides e2eScenarioVMOverride `yaml:"overrides"`
}

type e2eScenarioVMOverride struct {
	MemoryMB   int                   `yaml:"memoryMb"`
	ExtraDisks e2eScenarioExtraDisks `yaml:"extraDisks"`
}

type e2eScenarioExtraDisks struct {
	Count  int `yaml:"count"`
	SizeMB int `yaml:"sizeMb"`
}

type e2eProvisionEntry struct {
	Type      string
	Storage   e2eStorageProvision
	Bootstrap e2eVMProvision
	Worker    e2eVMProvision
}

type e2eStorageProvision struct {
	VM    string   `yaml:"vm"`
	Pool  string   `yaml:"pool"`
	VDevs []string `yaml:"vdevs"`
}

type e2eVMProvision struct {
	VM string `yaml:"vm"`
}

type e2eScenarioStepEntry struct {
	Type          string
	NodePrepISCSI e2eNodePrepISCSIStep
	HelmInstall   e2eHelmInstallStep
}

type e2eNodePrepISCSIStep struct {
	VMs []string `yaml:"vms"`
}

type e2eHelmInstallStep struct {
	Release      string `yaml:"release"`
	Chart        string `yaml:"chart"`
	Repo         string `yaml:"repo"`
	Namespace    string `yaml:"namespace"`
	ChartVersion string `yaml:"chartVersion"`
	Values       string `yaml:"values"`
}

type e2eScenarioCheck struct {
	Type           string
	PVCLifecycle   e2ePVCLifecycleCheck
	ZFSConsistency e2eVMCheck
	DeleteHygiene  e2eVMCheck
	NodesReady     e2eNodesReadyCheck
}

type e2ePVCLifecycleCheck struct {
	Namespace    string `yaml:"namespace"`
	Size         string `yaml:"size"`
	StorageClass string `yaml:"storageClass"`
}

type e2eVMCheck struct {
	VM string `yaml:"vm"`
}

type e2eNodesReadyCheck struct {
	Count int `yaml:"count"`
}

type e2eScenarioState struct {
	scratchDir       string
	vmIDs            map[string]string
	targets          map[string]testvm.ProvisionTarget
	operatorPub      string
	operatorPriv     string
	kubeconfig       string
	valuesPath       string
	csiKeyPath       string
	storageCreds     storageCredentials
	lastPVCNamespace string
	lastPVCSizeBytes int64
	helmReleases     []e2eHelmRelease
	results          []e2eCheckResult
	cleaned          bool
}

type e2eHelmRelease struct {
	name      string
	namespace string
}

type e2ePlanStep struct {
	Phase string
	Label string
	Kind  string
}

func (c *e2eListCmd) Run(rcx *runContext) error {
	scenarios, err := listE2EScenarios(rcx.repoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		rows = append(rows, []string{scenario.Name, scenario.Description})
	}
	reporter.Table([]string{"SCENARIO", "DESCRIPTION"}, rows)
	return nil
}

func (c *e2eRunCmd) Run(rcx *runContext) error {
	scenario, err := loadE2EScenario(rcx.repoRoot, c.Scenario)
	if err != nil {
		return err
	}
	return runE2EScenario(context.Background(), rcx, scenario, c.options())
}

func (c e2eRunCmd) options() e2eRunOptions {
	return e2eRunOptions{
		scenarioName:       c.Scenario,
		keep:               c.Keep,
		clusterTarget:      firstNonEmpty(c.ClusterTarget, "v3"),
		clusterName:        firstNonEmpty(c.ClusterName, "k2e2e"),
		namespace:          c.Namespace,
		skipTeardownOnFail: c.SkipTeardownOnFail,
	}
}

func e2eScenarioDir(repoRoot string) string {
	return filepath.Join(repoRoot, "kairos", "tools", "e2e-scenarios")
}

func listE2EScenarios(repoRoot string) ([]e2eScenario, error) {
	entries, err := os.ReadDir(e2eScenarioDir(repoRoot))
	if err != nil {
		return nil, err
	}
	var scenarios []e2eScenario
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		scenario, err := loadE2EScenarioPath(filepath.Join(e2eScenarioDir(repoRoot), entry.Name()))
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].Name < scenarios[j].Name })
	return scenarios, nil
}

func loadE2EScenario(repoRoot string, name string) (*e2eScenario, error) {
	name = strings.TrimSuffix(name, ".yaml")
	path := filepath.Join(e2eScenarioDir(repoRoot), name+".yaml")
	scenario, err := loadE2EScenarioPath(path)
	if err != nil {
		return nil, err
	}
	return &scenario, nil
}

func loadE2EScenarioPath(path string) (e2eScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return e2eScenario{}, fmt.Errorf("read e2e scenario %s: %w", path, err)
	}
	var scenario e2eScenario
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&scenario); err != nil {
		return e2eScenario{}, fmt.Errorf("decode e2e scenario %s: %w", path, err)
	}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := validateE2EScenario(stem, &scenario); err != nil {
		return e2eScenario{}, fmt.Errorf("validate e2e scenario %s: %w", path, err)
	}
	return scenario, nil
}

func validateE2EScenario(stem string, scenario *e2eScenario) error {
	if scenario.Name != stem {
		return fmt.Errorf("name %q must match filename stem %q", scenario.Name, stem)
	}
	if scenario.Description == "" {
		return fmt.Errorf("description is required")
	}
	vms := map[string]bool{}
	for _, vm := range scenario.VMs {
		if vm.Name == "" {
			return fmt.Errorf("vm name is required")
		}
		if vm.Preset == "" {
			return fmt.Errorf("vm %q preset is required", vm.Name)
		}
		if vms[vm.Name] {
			return fmt.Errorf("duplicate vm name %q", vm.Name)
		}
		vms[vm.Name] = true
	}
	for _, provision := range scenario.Provision {
		if err := validateE2EVMRef(vms, provision.vmName(), provision.Type); err != nil {
			return err
		}
	}
	for _, step := range scenario.Steps {
		switch step.Type {
		case "nodePrepISCSI":
			for _, vm := range step.NodePrepISCSI.VMs {
				if err := validateE2EVMRef(vms, vm, step.Type); err != nil {
					return err
				}
			}
		case "helmInstall":
			if !known(e2eValueGeneratorKeys, step.HelmInstall.Values) {
				return fmt.Errorf("unknown values generator %q (known: %s)", step.HelmInstall.Values, strings.Join(e2eValueGeneratorKeys, ", "))
			}
		}
	}
	for _, check := range scenario.Checks {
		switch check.Type {
		case "zfsConsistency":
			if err := validateE2EVMRef(vms, check.ZFSConsistency.VM, check.Type); err != nil {
				return err
			}
		case "deleteHygiene":
			if err := validateE2EVMRef(vms, check.DeleteHygiene.VM, check.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateE2EVMRef(vms map[string]bool, name string, owner string) error {
	if name == "" {
		return fmt.Errorf("%s vm is required", owner)
	}
	if !vms[name] {
		return fmt.Errorf("%s references undeclared vm %q", owner, name)
	}
	return nil
}

func runE2EScenario(parent context.Context, rcx *runContext, scenario *e2eScenario, opts e2eRunOptions) (err error) {
	applyE2ERunOverrides(scenario, opts)
	if err := validateE2EScenario(scenario.Name, scenario); err != nil {
		return err
	}
	scratchDir, err := os.MkdirTemp("", "k2-e2e-"+scenario.Name+"-*")
	if err != nil {
		return err
	}
	state := newE2EScenarioState(scenario, opts, scratchDir)
	defer func() {
		shouldCleanup := !opts.keep && (err == nil || !opts.skipTeardownOnFail)
		if shouldCleanup && !state.cleaned {
			if cleanupErr := cleanupE2EScenario(rcx, state, opts); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
		}
		printE2ESummary(state)
	}()

	reporter.Infof("scratch: %s", scratchDir)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	reporter.SetInterruptCancel(cancel)
	defer reporter.SetInterruptCancel(nil)

	wf := ui.NewWorkflow(reporter)
	wf.Section("Preflight")
	wf.KeyValues(
		ui.KV{Key: "Scenario", Value: scenario.Name},
		ui.KV{Key: "Scratch", Value: scratchDir},
		ui.KV{Key: "Cluster", Value: opts.clusterName},
		ui.KV{Key: "VMs", Value: strings.Join(e2eScenarioVMIDs(state), ", ")},
	)
	wf.Shell("Check local artifacts and tools", stepE2EPreflight(rcx, scenario))
	wf.Task("Generate e2e operator key", stepE2EGenerateOperatorKey(state))

	wf.Section("Create VMs")
	for _, vm := range scenario.VMs {
		vm := vm
		wf.Shell("Create/start VM "+state.vmIDs[vm.Name], stepE2EEnsureVM(rcx, state, vm))
	}
	for _, vm := range scenario.VMs {
		vm := vm
		wf.Shell("Wait for VM "+state.vmIDs[vm.Name], stepE2EWaitVM(rcx, state, vm.Name))
	}

	wf.Section("Provision")
	for _, entry := range scenario.Provision {
		entry := entry
		wf.Shell(e2eProvisionLabel(entry, state), stepE2EProvision(rcx, state, opts, entry))
	}

	wf.Section("Steps")
	for _, entry := range scenario.Steps {
		entry := entry
		wf.Shell(e2eStepLabel(entry), stepE2EScenarioStep(state, entry))
	}

	wf.Section("Checks")
	for _, entry := range scenario.Checks {
		entry := entry
		wf.Shell(e2eCheckLabel(entry), stepE2EScenarioCheck(state, entry))
	}

	wf.Section("Teardown").Unless(opts.keep)
	wf.Shell("Remove e2e resources", func(ctx context.Context, sh ui.Step) error {
		if err := cleanupE2EScenario(rcx, state, opts); err != nil {
			return err
		}
		state.cleaned = true
		sh.Successf("removed VMs, cluster credentials, and scratch")
		return nil
	}).Unless(opts.keep)

	return wf.Execute(ctx)
}

func applyE2ERunOverrides(scenario *e2eScenario, opts e2eRunOptions) {
	for i := range scenario.Steps {
		if scenario.Steps[i].Type == "helmInstall" && opts.chartVersion != "" {
			scenario.Steps[i].HelmInstall.ChartVersion = opts.chartVersion
		}
	}
	for i := range scenario.Checks {
		if scenario.Checks[i].Type != "pvcLifecycle" {
			continue
		}
		if opts.namespace != "" {
			scenario.Checks[i].PVCLifecycle.Namespace = opts.namespace
		}
		if opts.pvcSize != "" {
			scenario.Checks[i].PVCLifecycle.Size = opts.pvcSize
		}
	}
}

func newE2EScenarioState(scenario *e2eScenario, opts e2eRunOptions, scratchDir string) *e2eScenarioState {
	state := &e2eScenarioState{
		scratchDir: scratchDir,
		vmIDs:      map[string]string{},
		targets:    map[string]testvm.ProvisionTarget{},
	}
	base := sanitizeE2EName(opts.clusterName)
	for _, vm := range scenario.VMs {
		state.vmIDs[vm.Name] = "e2e-" + base + "-" + vm.Name
	}
	return state
}

func e2eScenarioVMIDs(s *e2eScenarioState) []string {
	ids := make([]string, 0, len(s.vmIDs))
	for _, id := range s.vmIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func stepE2EPreflight(rcx *runContext, scenario *e2eScenario) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		targets := map[string]bool{}
		for _, vm := range scenario.VMs {
			target, err := testvm.ResolvePresetArtifactTarget(rcx.repoRoot, vm.Preset)
			if err != nil {
				return err
			}
			targets[target] = true
		}
		for target := range targets {
			if !localArtifactExists(rcx.repoRoot, target) {
				return missingArtifactError(target)
			}
		}
		for _, bin := range e2eRequiredTools(scenario) {
			if _, err := exec.LookPath(bin); err != nil {
				return fmt.Errorf("%s binary not found on PATH: %w", bin, err)
			}
		}
		if !sudoQEMU() {
			sh.Warnf("qemu-vmnet requires sudo QEMU; set K2_TOOLS_VM_SUDO_QEMU=1 for the live run")
			return nil
		}
		sh.Successf("artifacts and tools available")
		return nil
	}
}

func e2eRequiredTools(scenario *e2eScenario) []string {
	tools := map[string]bool{}
	for _, step := range scenario.Steps {
		if step.Type == "helmInstall" {
			tools["helm"] = true
		}
	}
	for _, check := range scenario.Checks {
		switch check.Type {
		case "pvcLifecycle", "zfsConsistency", "deleteHygiene", "nodesReady":
			tools["kubectl"] = true
		}
	}
	out := make([]string, 0, len(tools))
	for tool := range tools {
		out = append(out, tool)
	}
	sort.Strings(out)
	return out
}

func stepE2EGenerateOperatorKey(s *e2eScenarioState) func(context.Context) error {
	return func(ctx context.Context) error {
		priv, pub, _, err := writeE2EKeyPair(s.scratchDir)
		if err != nil {
			return err
		}
		s.operatorPriv = priv
		s.operatorPub = pub
		return nil
	}
}

func stepE2EEnsureVM(rcx *runContext, s *e2eScenarioState, vm e2eScenarioVM) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		id := s.vmIDs[vm.Name]
		runner := testvm.Runner{RepoRoot: rcx.repoRoot, Reporter: reporter}
		if e2eVMExists(rcx.repoRoot, id) {
			if err := runner.Start(testvm.StartOptions{ID: id, Sudo: sudoQEMU()}); err != nil {
				return err
			}
			sh.Successf("reusing existing VM %s", id)
			return nil
		}
		opts := testvm.CreateOptions{
			Preset:          vm.Preset,
			ID:              id,
			MemoryMB:        vm.Overrides.MemoryMB,
			ExtraDisks:      vm.Overrides.ExtraDisks.Count,
			ExtraDiskSizeMB: vm.Overrides.ExtraDisks.SizeMB,
			Start:           true,
			Sudo:            sudoQEMU(),
		}
		if err := runner.Create(opts); err != nil {
			return err
		}
		sh.Successf("VM %s ready", id)
		return nil
	}
}

func stepE2EWaitVM(rcx *runContext, s *e2eScenarioState, name string) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		id := s.vmIDs[name]
		target, err := waitProvisionTarget(ctx, rcx.repoRoot, id, 5*time.Minute)
		if err != nil {
			return err
		}
		client := remote.Client{Host: target.Host, Port: target.Port, User: "kairos", Stdout: sh, Stderr: sh, Logger: shLogf(sh)}
		if err := client.WaitForAuth(5 * time.Minute); err != nil {
			return err
		}
		s.targets[name] = target
		sh.Successf("%s reachable at %s:%d (%s)", id, target.Host, target.Port, target.GuestIPv4.Address)
		return nil
	}
}

func stepE2EProvision(rcx *runContext, s *e2eScenarioState, opts e2eRunOptions, entry e2eProvisionEntry) func(context.Context, ui.Step) error {
	switch entry.Type {
	case "storage":
		return stepE2EProvisionStorage(rcx, s, opts, entry.Storage)
	case "bootstrap":
		return stepE2EProvisionBootstrap(rcx, s, opts, entry.Bootstrap.VM)
	case "worker":
		return stepE2EProvisionWorker(rcx, s, opts, entry.Worker.VM)
	default:
		return func(context.Context, ui.Step) error { return fmt.Errorf("unknown provision type %q", entry.Type) }
	}
}

func stepE2EProvisionStorage(rcx *runContext, s *e2eScenarioState, opts e2eRunOptions, provision e2eStorageProvision) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		id := s.vmIDs[provision.VM]
		target := s.targets[provision.VM]
		cmd := storageCmd{
			commonStorageFlags: commonStorageFlags{
				ClusterTarget:     opts.clusterTarget,
				ClusterName:       opts.clusterName,
				NodeName:          id,
				Pool:              firstNonEmpty(provision.Pool, "tank"),
				PoolVDev:          provision.VDevs,
				OperatorFiles:     []string{s.operatorPub},
				IQNBase:           "iqn.2026-07.io.wyvernzora.k2:storage",
				PoolCompatibility: "openzfs-2.2-linux",
			},
			TestVM:   id,
			Host:     target.Host,
			SSHPort:  target.Port,
			SSHUser:  "kairos",
			Identity: s.operatorPriv,
			Yes:      true,
		}
		if _, err := runStorageProvision(ctx, rcx, &cmd); err != nil {
			return err
		}
		creds, ok := loadStorageCredentials(opts.clusterName)
		if !ok {
			return fmt.Errorf("storage credentials not found after provisioning")
		}
		s.storageCreds = creds
		sh.Successf("storage credentials written for %s", creds.Portal)
		return nil
	}
}

func stepE2EProvisionBootstrap(rcx *runContext, s *e2eScenarioState, opts e2eRunOptions, vmName string) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		id := s.vmIDs[vmName]
		cmd := bootstrapCmd{
			commonBootstrapFlags: commonBootstrapFlags{
				ClusterTarget: opts.clusterTarget,
				ClusterName:   opts.clusterName,
				NodeName:      id,
				OperatorFiles: []string{s.operatorPub},
			},
			TestVM:  id,
			SSHUser: "kairos",
			Yes:     true,
		}
		if err := runBootstrapProvision(ctx, rcx, &cmd); err != nil {
			return err
		}
		kubeconfig, err := kubeconfigPathFor(opts.clusterName)
		if err != nil {
			return err
		}
		s.kubeconfig = kubeconfig
		sh.Successf("kubeconfig %s", kubeconfig)
		return nil
	}
}

func stepE2EProvisionWorker(rcx *runContext, s *e2eScenarioState, opts e2eRunOptions, vmName string) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		id := s.vmIDs[vmName]
		err := runJoinProvision(ctx, rcx, nodeRoleWorker,
			commonJoinFlags{
				ClusterTarget: opts.clusterTarget,
				ClusterName:   opts.clusterName,
				NodeName:      id,
				OperatorFiles: []string{s.operatorPub},
			},
			commonRemoteFlags{
				TestVM:  id,
				SSHUser: "kairos",
				Yes:     true,
			},
		)
		if err != nil {
			return err
		}
		sh.Successf("worker %s provisioned", id)
		return nil
	}
}

func stepE2EScenarioStep(s *e2eScenarioState, entry e2eScenarioStepEntry) func(context.Context, ui.Step) error {
	switch entry.Type {
	case "nodePrepISCSI":
		return stepE2ENodePrepISCSI(s, entry.NodePrepISCSI)
	case "helmInstall":
		return stepE2EHelmInstall(s, entry.HelmInstall)
	default:
		return func(context.Context, ui.Step) error { return fmt.Errorf("unknown step type %q", entry.Type) }
	}
}

func stepE2ENodePrepISCSI(s *e2eScenarioState, step e2eNodePrepISCSIStep) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		for _, vm := range step.VMs {
			target := s.targets[vm]
			client := remote.Client{Host: target.Host, Port: target.Port, User: "kairos", IdentityFile: s.operatorPriv, Stdout: sh, Stderr: sh, Logger: shLogf(sh)}
			if err := client.Run(e2eNodeISCSIPrepScript()); err != nil {
				return fmt.Errorf("prepare iSCSI on %s: %w", s.vmIDs[vm], err)
			}
		}
		sh.Successf("iSCSI initiators prepared")
		return nil
	}
}

func stepE2EHelmInstall(s *e2eScenarioState, step e2eHelmInstallStep) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		values, err := e2eValuesYAML(step.Values, s.storageCreds)
		if err != nil {
			return err
		}
		s.valuesPath = filepath.Join(s.scratchDir, step.Release+"-values.yaml")
		if err := os.WriteFile(s.valuesPath, values, 0o600); err != nil {
			return err
		}
		s.csiKeyPath = filepath.Join(s.scratchDir, "csi_ed25519")
		if err := os.WriteFile(s.csiKeyPath, []byte(s.storageCreds.CSIPrivateKey), 0o600); err != nil {
			return err
		}
		repoName := strings.Split(step.Chart, "/")[0]
		if _, err := runExternal(ctx, sh, sh, nil, e2eKubeEnv(s.kubeconfig), "helm", "repo", "add", repoName, step.Repo, "--force-update"); err != nil {
			return err
		}
		if _, err := runExternal(ctx, sh, sh, nil, e2eKubeEnv(s.kubeconfig), "helm", "repo", "update", repoName); err != nil {
			return err
		}
		namespace := firstNonEmpty(step.Namespace, step.Release)
		args := []string{"upgrade", "--install", step.Release, step.Chart, "-n", namespace, "--create-namespace", "-f", s.valuesPath}
		if step.ChartVersion != "" {
			args = append(args, "--version", step.ChartVersion)
		}
		if _, err := runExternal(ctx, sh, sh, nil, e2eKubeEnv(s.kubeconfig), "helm", args...); err != nil {
			return err
		}
		s.helmReleases = append(s.helmReleases, e2eHelmRelease{name: step.Release, namespace: namespace})
		sh.Successf("%s installed", step.Release)
		return nil
	}
}

func e2eValuesYAML(name string, creds storageCredentials) ([]byte, error) {
	switch name {
	case "storageDriverValues":
		return democraticCSIValuesYAML(creds)
	default:
		return nil, fmt.Errorf("unknown values generator %q (known: %s)", name, strings.Join(e2eValueGeneratorKeys, ", "))
	}
}

func stepE2EScenarioCheck(s *e2eScenarioState, entry e2eScenarioCheck) func(context.Context, ui.Step) error {
	switch entry.Type {
	case "pvcLifecycle":
		return recordedE2ECheck(s, "pvcLifecycle", stepE2EPVCLifecycle(s, entry.PVCLifecycle))
	case "zfsConsistency":
		return recordedE2ECheck(s, "zfsConsistency", stepE2EZFSConsistency(s, entry.ZFSConsistency))
	case "deleteHygiene":
		return recordedE2ECheck(s, "deleteHygiene", stepE2EDeleteHygiene(s, entry.DeleteHygiene))
	case "nodesReady":
		return recordedE2ECheck(s, "nodesReady", stepE2ENodesReady(s, entry.NodesReady))
	default:
		return func(context.Context, ui.Step) error { return fmt.Errorf("unknown check type %q", entry.Type) }
	}
}

func stepE2EPVCLifecycle(s *e2eScenarioState, check e2ePVCLifecycleCheck) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		expectedBytes, err := parseSimpleQuantityBytes(check.Size)
		if err != nil {
			return err
		}
		s.lastPVCNamespace = check.Namespace
		s.lastPVCSizeBytes = expectedBytes
		manifest, err := e2eAcceptanceManifest(check.Namespace, check.Size, check.StorageClass)
		if err != nil {
			return err
		}
		if _, err := runKubectl(ctx, s.kubeconfig, sh, sh, manifest, "apply", "-f", "-"); err != nil {
			return err
		}
		if err := waitKubectlEquals(ctx, s.kubeconfig, check.Namespace, "pvc", e2ePVCName, "{.status.phase}", "Bound", 5*time.Minute, sh); err != nil {
			_, _ = runKubectl(ctx, s.kubeconfig, sh, sh, nil, "-n", check.Namespace, "describe", "pvc", e2ePVCName)
			return err
		}
		if err := waitKubectlEquals(ctx, s.kubeconfig, check.Namespace, "pod", e2ePodName, "{.status.phase}", "Running", 5*time.Minute, sh); err != nil {
			_, _ = runKubectl(ctx, s.kubeconfig, sh, sh, nil, "-n", check.Namespace, "describe", "pod", e2ePodName)
			return err
		}
		script := e2eIOCheckScript("k2 e2e storage acceptance\n")
		if _, err := runKubectl(ctx, s.kubeconfig, sh, sh, nil, "-n", check.Namespace, "exec", e2ePodName, "--", "sh", "-lc", script); err != nil {
			return err
		}
		sh.Successf("PVC Bound, pod Running, and data checksum matched")
		return nil
	}
}

func stepE2EZFSConsistency(s *e2eScenarioState, check e2eVMCheck) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		if s.lastPVCSizeBytes == 0 {
			return fmt.Errorf("zfsConsistency requires pvcLifecycle to run first")
		}
		pvBytes, err := runKubectl(ctx, s.kubeconfig, nil, sh, nil, "-n", s.lastPVCNamespace, "get", "pvc", e2ePVCName, "-o", "jsonpath={.spec.volumeName}")
		if err != nil {
			return err
		}
		pvName := strings.TrimSpace(string(pvBytes))
		if pvName == "" {
			return fmt.Errorf("PVC %s has no bound PV name", e2ePVCName)
		}
		client := remote.Client{
			Host:         s.storageCreds.SSHHost,
			Port:         s.storageCreds.SSHPort,
			User:         s.storageCreds.SSHUser,
			IdentityFile: s.csiKeyPath,
			Stdout:       sh,
			Stderr:       sh,
			Logger:       shLogf(sh),
		}
		if err := client.Run(e2eStorageConsistencyScript(s.storageCreds, pvName, s.lastPVCSizeBytes)); err != nil {
			return err
		}
		sh.Successf("zvol, encryption, keystatus, and LIO target match %s", pvName)
		return nil
	}
}

func stepE2EDeleteHygiene(s *e2eScenarioState, check e2eVMCheck) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		pvBytes, _ := runKubectl(ctx, s.kubeconfig, nil, sh, nil, "-n", s.lastPVCNamespace, "get", "pvc", e2ePVCName, "-o", "jsonpath={.spec.volumeName}")
		pvName := strings.TrimSpace(string(pvBytes))
		_, _ = runKubectl(ctx, s.kubeconfig, sh, sh, nil, "-n", s.lastPVCNamespace, "delete", "pod", e2ePodName, "--ignore-not-found=true", "--wait=true")
		_, _ = runKubectl(ctx, s.kubeconfig, sh, sh, nil, "-n", s.lastPVCNamespace, "delete", "pvc", e2ePVCName, "--ignore-not-found=true", "--wait=true")
		client := remote.Client{Host: s.storageCreds.SSHHost, Port: s.storageCreds.SSHPort, User: s.storageCreds.SSHUser, IdentityFile: s.csiKeyPath, Stdout: sh, Stderr: sh, Logger: shLogf(sh)}
		if pvName != "" {
			if err := client.Run(e2eStorageCleanupPollScript(s.storageCreds, pvName)); err != nil {
				return err
			}
		}
		for vm, target := range s.targets {
			if vm == check.VM {
				continue
			}
			node := remote.Client{Host: target.Host, Port: target.Port, User: "kairos", IdentityFile: s.operatorPriv, Stdout: sh, Stderr: sh, Logger: shLogf(sh)}
			if err := node.Run(e2eNodeNoISCSISessionScript(s.storageCreds.IQNBase)); err != nil {
				return fmt.Errorf("node %s still has iSCSI session for %s: %w", s.vmIDs[vm], s.storageCreds.IQNBase, err)
			}
		}
		sh.Successf("PVC, zvol, target, and sessions cleaned up")
		return nil
	}
}

func stepE2ENodesReady(s *e2eScenarioState, check e2eNodesReadyCheck) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		deadline := time.Now().Add(5 * time.Minute)
		for {
			ready, total, err := e2eReadyNodeCount(ctx, s.kubeconfig, sh)
			if err == nil && ready == check.Count && total == check.Count {
				sh.Successf("%d/%d nodes Ready", ready, total)
				return nil
			}
			if time.Now().After(deadline) {
				if err != nil {
					return err
				}
				return fmt.Errorf("timed out waiting for %d Ready nodes; last ready=%d total=%d", check.Count, ready, total)
			}
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
}

type e2eNodeList struct {
	Items []struct {
		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

func e2eReadyNodeCount(ctx context.Context, kubeconfig string, out io.Writer) (int, int, error) {
	data, err := runKubectl(ctx, kubeconfig, nil, out, nil, "get", "nodes", "-o", "json")
	if err != nil {
		return 0, 0, err
	}
	var list e2eNodeList
	if err := json.Unmarshal(data, &list); err != nil {
		return 0, 0, err
	}
	ready := 0
	for _, node := range list.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready++
				break
			}
		}
	}
	return ready, len(list.Items), nil
}

func recordedE2ECheck(s *e2eScenarioState, name string, fn func(context.Context, ui.Step) error) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		if err := fn(ctx, sh); err != nil {
			s.results = append(s.results, e2eCheckResult{Name: name, Status: "FAIL", Detail: err.Error()})
			return err
		}
		s.results = append(s.results, e2eCheckResult{Name: name, Status: "PASS"})
		return nil
	}
}

func cleanupE2EScenario(rcx *runContext, s *e2eScenarioState, opts e2eRunOptions) error {
	var errs []error
	if s.kubeconfig != "" {
		for _, release := range s.helmReleases {
			if _, err := runExternal(context.Background(), io.Discard, io.Discard, nil, e2eKubeEnv(s.kubeconfig), "helm", "uninstall", release.name, "-n", release.namespace); err != nil {
				errs = append(errs, err)
			}
		}
	}
	runner := testvm.Runner{RepoRoot: rcx.repoRoot, Reporter: reporter}
	for _, id := range e2eScenarioVMIDs(s) {
		if !e2eVMExists(rcx.repoRoot, id) {
			continue
		}
		if err := runner.Delete(testvm.DeleteOptions{ID: id, Force: true}); err != nil {
			errs = append(errs, err)
		}
	}
	if dir, err := clusterCredentialsDir(opts.clusterName); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, err)
		}
	}
	if err := os.RemoveAll(s.scratchDir); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func printE2ESummary(s *e2eScenarioState) {
	if len(s.results) == 0 {
		return
	}
	rows := make([][]string, len(s.results))
	for i, result := range s.results {
		rows[i] = []string{result.Name, result.Status, result.Detail}
	}
	reporter.Table([]string{"CHECK", "STATUS", "DETAIL"}, rows)
}

func e2eExecutionPlan(scenario *e2eScenario, opts e2eRunOptions) ([]e2ePlanStep, error) {
	clone := *scenario
	clone.VMs = append([]e2eScenarioVM(nil), scenario.VMs...)
	clone.Provision = append([]e2eProvisionEntry(nil), scenario.Provision...)
	clone.Steps = append([]e2eScenarioStepEntry(nil), scenario.Steps...)
	clone.Checks = append([]e2eScenarioCheck(nil), scenario.Checks...)
	applyE2ERunOverrides(&clone, opts)
	if err := validateE2EScenario(clone.Name, &clone); err != nil {
		return nil, err
	}
	state := newE2EScenarioState(&clone, opts, "/tmp/k2-e2e-plan")
	steps := []e2ePlanStep{
		{Phase: "Preflight", Label: "Check local artifacts and tools", Kind: "preflight"},
		{Phase: "Preflight", Label: "Generate e2e operator key", Kind: "operatorKey"},
	}
	for _, vm := range clone.VMs {
		steps = append(steps, e2ePlanStep{Phase: "Create VMs", Label: "Create/start VM " + state.vmIDs[vm.Name], Kind: "vm"})
	}
	for _, vm := range clone.VMs {
		steps = append(steps, e2ePlanStep{Phase: "Create VMs", Label: "Wait for VM " + state.vmIDs[vm.Name], Kind: "vmWait"})
	}
	for _, entry := range clone.Provision {
		steps = append(steps, e2ePlanStep{Phase: "Provision", Label: e2eProvisionLabel(entry, state), Kind: entry.Type})
	}
	for _, entry := range clone.Steps {
		steps = append(steps, e2ePlanStep{Phase: "Steps", Label: e2eStepLabel(entry), Kind: entry.Type})
	}
	for _, entry := range clone.Checks {
		steps = append(steps, e2ePlanStep{Phase: "Checks", Label: e2eCheckLabel(entry), Kind: entry.Type})
	}
	steps = append(steps, e2ePlanStep{Phase: "Teardown", Label: "Remove e2e resources", Kind: "teardown"})
	return steps, nil
}

func e2eProvisionLabel(entry e2eProvisionEntry, s *e2eScenarioState) string {
	switch entry.Type {
	case "storage":
		return "Provision storage appliance " + s.vmIDs[entry.Storage.VM]
	case "bootstrap":
		return "Provision bootstrap server " + s.vmIDs[entry.Bootstrap.VM]
	case "worker":
		return "Provision worker " + s.vmIDs[entry.Worker.VM]
	default:
		return "Provision " + entry.Type
	}
}

func e2eStepLabel(entry e2eScenarioStepEntry) string {
	switch entry.Type {
	case "nodePrepISCSI":
		return "Prepare node iSCSI initiators"
	case "helmInstall":
		return "Install " + entry.HelmInstall.Release
	default:
		return entry.Type
	}
}

func e2eCheckLabel(entry e2eScenarioCheck) string {
	switch entry.Type {
	case "pvcLifecycle":
		return "PVC lifecycle"
	case "zfsConsistency":
		return "ZFS consistency"
	case "deleteHygiene":
		return "Delete hygiene"
	case "nodesReady":
		return "Nodes Ready"
	default:
		return entry.Type
	}
}

func (e *e2eProvisionEntry) vmName() string {
	switch e.Type {
	case "storage":
		return e.Storage.VM
	case "bootstrap":
		return e.Bootstrap.VM
	case "worker":
		return e.Worker.VM
	default:
		return ""
	}
}

func (e *e2eProvisionEntry) UnmarshalYAML(value *yaml.Node) error {
	key, node, err := oneKeyEntry(value, "provision", e2eProvisionTypes)
	if err != nil {
		return err
	}
	e.Type = key
	switch key {
	case "storage":
		return decodeKnownYAML(node, &e.Storage, "storage", []string{"vm", "pool", "vdevs"})
	case "bootstrap":
		return decodeKnownYAML(node, &e.Bootstrap, "bootstrap", []string{"vm"})
	case "worker":
		return decodeKnownYAML(node, &e.Worker, "worker", []string{"vm"})
	default:
		return fmt.Errorf("unknown provision type %q (known: %s)", key, strings.Join(e2eProvisionTypes, ", "))
	}
}

func (e *e2eScenarioStepEntry) UnmarshalYAML(value *yaml.Node) error {
	key, node, err := oneKeyEntry(value, "step", e2eStepTypes)
	if err != nil {
		return err
	}
	e.Type = key
	switch key {
	case "nodePrepISCSI":
		return decodeKnownYAML(node, &e.NodePrepISCSI, "nodePrepISCSI", []string{"vms"})
	case "helmInstall":
		return decodeKnownYAML(node, &e.HelmInstall, "helmInstall", []string{"release", "chart", "repo", "namespace", "chartVersion", "values"})
	default:
		return fmt.Errorf("unknown step type %q (known: %s)", key, strings.Join(e2eStepTypes, ", "))
	}
}

func (e *e2eScenarioCheck) UnmarshalYAML(value *yaml.Node) error {
	key, node, err := oneKeyEntry(value, "check", e2eCheckTypes)
	if err != nil {
		return err
	}
	e.Type = key
	switch key {
	case "pvcLifecycle":
		return decodeKnownYAML(node, &e.PVCLifecycle, "pvcLifecycle", []string{"namespace", "size", "storageClass"})
	case "zfsConsistency":
		return decodeKnownYAML(node, &e.ZFSConsistency, "zfsConsistency", []string{"vm"})
	case "deleteHygiene":
		return decodeKnownYAML(node, &e.DeleteHygiene, "deleteHygiene", []string{"vm"})
	case "nodesReady":
		return decodeKnownYAML(node, &e.NodesReady, "nodesReady", []string{"count"})
	default:
		return fmt.Errorf("unknown check type %q (known: %s)", key, strings.Join(e2eCheckTypes, ", "))
	}
}

func oneKeyEntry(value *yaml.Node, kind string, knownKeys []string) (string, *yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return "", nil, fmt.Errorf("%s entry must be a mapping", kind)
	}
	if len(value.Content) != 2 {
		return "", nil, fmt.Errorf("%s entry must have exactly one key", kind)
	}
	key := value.Content[0].Value
	if !known(knownKeys, key) {
		return "", nil, fmt.Errorf("unknown %s type %q (known: %s)", kind, key, strings.Join(knownKeys, ", "))
	}
	return key, value.Content[1], nil
}

func decodeKnownYAML(node *yaml.Node, out any, kind string, fields []string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s value must be a mapping", kind)
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !known(fields, key) {
			return fmt.Errorf("unknown %s field %q (known: %s)", kind, key, strings.Join(fields, ", "))
		}
	}
	return node.Decode(out)
}

func known(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
