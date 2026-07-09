package toolcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testvm "github.com/wyvernzora/k2/tools/internal/kairos/tools/vm"
)

type e2eStorageCmd struct {
	Keep               bool   `name:"keep" help:"Preserve VMs and scratch state on exit or failure."`
	Workers            int    `name:"workers" default:"1" help:"Number of worker VMs."`
	ClusterTarget      string `name:"cluster-target" default:"v3" help:"Cluster config/deploy target."`
	ClusterName        string `name:"cluster-name" default:"k2e2e" help:"Local e2e cluster name."`
	Namespace          string `name:"namespace" default:"e2e-storage" help:"Namespace for PVC acceptance checks."`
	PVCSize            string `name:"pvc-size" default:"1Gi" help:"PVC size for acceptance checks."`
	ChartVersion       string `name:"chart-version" help:"Optional democratic-csi chart version pin."`
	SkipTeardownOnFail bool   `name:"skip-teardown-on-fail" help:"Preserve VMs and scratch state only when the harness fails."`
}

type e2eCheckResult struct {
	Name   string
	Status string
	Detail string
}

func (c *e2eStorageCmd) Run(rcx *runContext) error {
	*c = c.defaults()
	if c.Workers < 0 {
		return fmt.Errorf("workers must be >= 0")
	}
	scenario, err := loadE2EScenario(rcx.repoRoot, storagePVCScenarioName)
	if err != nil {
		return err
	}
	applyStorageAliasOverrides(scenario, *c)
	if err := preauthorizeSudoQEMU(); err != nil {
		return err
	}
	return runE2EScenario(context.Background(), rcx, scenario, c.options())
}

func (c e2eStorageCmd) options() e2eRunOptions {
	return e2eRunOptions{
		scenarioName:       storagePVCScenarioName,
		keep:               c.Keep,
		clusterTarget:      firstNonEmpty(c.ClusterTarget, "v3"),
		clusterName:        firstNonEmpty(c.ClusterName, "k2e2e"),
		namespace:          c.Namespace,
		pvcSize:            c.PVCSize,
		chartVersion:       c.ChartVersion,
		skipTeardownOnFail: c.SkipTeardownOnFail,
	}
}

func applyStorageAliasOverrides(scenario *e2eScenario, c e2eStorageCmd) {
	if c.Workers != 1 {
		scenario.VMs = []e2eScenarioVM{
			{Name: "storage", Preset: e2eStoragePreset, Overrides: e2eScenarioVMOverride{ExtraDisks: e2eScenarioExtraDisks{Count: 2, SizeMB: 4096}}},
			{Name: "server", Preset: e2eK8sPreset},
		}
		scenario.Provision = []e2eProvisionEntry{
			{Type: "storage", Storage: e2eStorageProvision{VM: "storage", Pool: "tank", VDevs: []string{"mirror /dev/vdc /dev/vdd"}}},
			{Type: "bootstrap", Bootstrap: e2eVMProvision{VM: "server"}},
		}
		nodes := []string{"server"}
		for i := range c.Workers {
			name := fmt.Sprintf("worker-%d", i+1)
			scenario.VMs = append(scenario.VMs, e2eScenarioVM{Name: name, Preset: e2eK8sPreset})
			scenario.Provision = append(scenario.Provision, e2eProvisionEntry{Type: "worker", Worker: e2eVMProvision{VM: name}})
			nodes = append(nodes, name)
		}
		for i := range scenario.Steps {
			if scenario.Steps[i].Type == "nodePrepISCSI" {
				scenario.Steps[i].NodePrepISCSI.VMs = nodes
			}
		}
	}
	applyE2ERunOverrides(scenario, c.options())
}

func waitProvisionTarget(ctx context.Context, repoRoot string, id string, timeout time.Duration) (testvm.ProvisionTarget, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		target, err := testvm.ResolveProvisionTarget(repoRoot, id)
		if err == nil && target.GuestIPv4.Address != "" {
			return target, nil
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return testvm.ProvisionTarget{}, fmt.Errorf("timed out waiting for VM %s guest IP: %w", id, lastErr)
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return testvm.ProvisionTarget{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func waitKubectlEquals(ctx context.Context, kubeconfig string, namespace string, kind string, name string, jsonpath string, want string, timeout time.Duration, out io.Writer) error {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		got, err := runKubectl(ctx, kubeconfig, nil, out, nil, "-n", namespace, "get", kind, name, "-o", "jsonpath="+jsonpath)
		if err == nil {
			last = strings.TrimSpace(string(got))
			if last == want {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s/%s %s=%s; last value %q", kind, name, jsonpath, want, last)
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

func runKubectl(ctx context.Context, kubeconfig string, stdout io.Writer, stderr io.Writer, stdin []byte, args ...string) ([]byte, error) {
	return runExternal(ctx, stdout, stderr, stdin, e2eKubeEnv(kubeconfig), "kubectl", args...)
}

func runExternal(ctx context.Context, stdout io.Writer, stderr io.Writer, stdin []byte, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out bytes.Buffer
	if stdout == nil {
		cmd.Stdout = &out
	} else {
		cmd.Stdout = io.MultiWriter(stdout, &out)
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out.Bytes(), nil
}

func e2eKubeEnv(kubeconfig string) []string {
	if kubeconfig == "" {
		return nil
	}
	return []string{"KUBECONFIG=" + kubeconfig}
}

func e2eVMExists(repoRoot string, id string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, ".testvm", "vm-"+id, "vm.json"))
	return err == nil
}

func sudoQEMU() bool {
	switch strings.ToLower(os.Getenv("K2_TOOLS_VM_SUDO_QEMU")) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func shLogf(sh io.Writer) func(string, ...any) {
	return func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		_, _ = io.WriteString(sh, line)
	}
}
