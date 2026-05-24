package kubectl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeRunner records each invocation + returns canned bytes per
// arg-prefix. Map key is a space-joined prefix of args[1:] (the
// kubectl args after the binary name). The first matching prefix
// wins.
type fakeRunner struct {
	out  map[string]string // arg prefix -> stdout
	err  map[string]error  // arg prefix -> error
	last []string
}

func (f *fakeRunner) run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
	f.last = args
	joined := strings.Join(args[1:], " ")
	for prefix, out := range f.out {
		if strings.HasPrefix(joined, prefix) {
			_, _ = stdout.Write([]byte(out))
			break
		}
	}
	for prefix, err := range f.err {
		if strings.HasPrefix(joined, prefix) {
			if stderr != nil {
				_, _ = stderr.Write([]byte("simulated stderr\n"))
			}
			return err
		}
	}
	return nil
}

const threeNodeJSON = `{
  "items": [
    {
      "metadata": {
        "name": "k2-pi-335e",
        "labels": {
          "node-role.kubernetes.io/control-plane": "true",
          "kubernetes.io/hostname": "k2-pi-335e"
        }
      },
      "spec": {"unschedulable": false},
      "status": {
        "addresses": [
          {"type": "InternalIP", "address": "10.10.9.10"},
          {"type": "Hostname", "address": "k2-pi-335e"}
        ],
        "conditions": [
          {"type": "Ready", "status": "True"}
        ]
      }
    },
    {
      "metadata": {
        "name": "k2-worker-01",
        "labels": {"kubernetes.io/hostname": "k2-worker-01"}
      },
      "spec": {"unschedulable": true},
      "status": {
        "addresses": [
          {"type": "InternalIP", "address": "10.10.9.20"}
        ],
        "conditions": [
          {"type": "Ready", "status": "False"}
        ]
      }
    },
    {
      "metadata": {
        "name": "k2-master-legacy",
        "labels": {"node-role.kubernetes.io/master": "true"}
      },
      "spec": {"unschedulable": false},
      "status": {
        "addresses": [
          {"type": "InternalIP", "address": "10.10.9.30"}
        ],
        "conditions": [
          {"type": "Ready", "status": "True"}
        ]
      }
    }
  ]
}`

func TestNodesParseMixedRoles(t *testing.T) {
	c := New("/dev/null/kubeconfig")
	c.SetRunnerForTest(&fakeRunner{
		out: map[string]string{
			"--kubeconfig=/dev/null/kubeconfig get nodes -o json": threeNodeJSON,
		},
	})
	nodes, err := c.Nodes(context.Background())
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}

	// First: CP, Ready, schedulable.
	if !nodes[0].IsControlPlane() {
		t.Error("nodes[0] should be a control plane")
	}
	if !nodes[0].Ready() {
		t.Error("nodes[0] should be Ready")
	}
	if !nodes[0].Schedulable {
		t.Error("nodes[0] should be schedulable")
	}
	if got := nodes[0].InternalIP(); got != "10.10.9.10" {
		t.Errorf("nodes[0] InternalIP = %q, want 10.10.9.10", got)
	}

	// Second: worker, NotReady, unschedulable (already cordoned).
	if nodes[1].IsControlPlane() {
		t.Error("nodes[1] should NOT be a control plane")
	}
	if nodes[1].Ready() {
		t.Error("nodes[1] should NOT be Ready")
	}
	if nodes[1].Schedulable {
		t.Error("nodes[1] should NOT be schedulable")
	}

	// Third: legacy master label, still recognized as CP.
	if !nodes[2].IsControlPlane() {
		t.Error("nodes[2] (legacy master label) should be recognized as control plane")
	}
}

func TestFindNodeByInternalIPMatchesExactlyOne(t *testing.T) {
	c := New("/dev/null/kubeconfig")
	c.SetRunnerForTest(&fakeRunner{
		out: map[string]string{
			"--kubeconfig=/dev/null/kubeconfig get nodes -o json": threeNodeJSON,
		},
	})
	got, err := c.FindNodeByInternalIP(context.Background(), "10.10.9.20")
	if err != nil {
		t.Fatalf("FindNodeByInternalIP: %v", err)
	}
	if got.Name != "k2-worker-01" {
		t.Errorf("got node name %q, want k2-worker-01", got.Name)
	}
}

func TestFindNodeByInternalIPErrorsWhenAbsent(t *testing.T) {
	c := New("/dev/null/kubeconfig")
	c.SetRunnerForTest(&fakeRunner{
		out: map[string]string{
			"--kubeconfig=/dev/null/kubeconfig get nodes -o json": threeNodeJSON,
		},
	})
	_, err := c.FindNodeByInternalIP(context.Background(), "10.99.99.99")
	if err == nil {
		t.Fatal("expected error when InternalIP doesn't match any node")
	}
}

func TestCordonInvokesKubectlWithKubeconfig(t *testing.T) {
	fr := &fakeRunner{}
	c := New("/etc/k2/kubeconfig")
	c.SetRunnerForTest(fr)
	if err := c.Cordon(context.Background(), "k2-pi-335e"); err != nil {
		t.Fatalf("Cordon: %v", err)
	}
	want := []string{"kubectl", "--kubeconfig=/etc/k2/kubeconfig", "cordon", "k2-pi-335e"}
	if !equalSlices(fr.last, want) {
		t.Errorf("got args=%v, want %v", fr.last, want)
	}
}

func TestUncordonInvokesKubectlWithKubeconfig(t *testing.T) {
	fr := &fakeRunner{}
	c := New("/etc/k2/kubeconfig")
	c.SetRunnerForTest(fr)
	if err := c.Uncordon(context.Background(), "k2-pi-335e"); err != nil {
		t.Fatalf("Uncordon: %v", err)
	}
	want := []string{"kubectl", "--kubeconfig=/etc/k2/kubeconfig", "uncordon", "k2-pi-335e"}
	if !equalSlices(fr.last, want) {
		t.Errorf("got args=%v, want %v", fr.last, want)
	}
}

func TestDrainBuildsArgsFromOpts(t *testing.T) {
	fr := &fakeRunner{}
	var stdout bytes.Buffer
	c := New("/etc/k2/kubeconfig")
	c.Stdout = &stdout
	c.SetRunnerForTest(fr)
	opts := DrainOpts{
		Timeout:            300_000_000_000, // 5min in ns
		IgnoreDaemonsets:   true,
		DeleteEmptyDirData: true,
		GracePeriodSeconds: -1, // pass-through kubectl default
	}
	if err := c.Drain(context.Background(), "k2-pi-335e", opts); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	joined := strings.Join(fr.last, " ")
	for _, want := range []string{
		"drain", "k2-pi-335e", "--ignore-daemonsets", "--delete-emptydir-data", "--timeout=5m0s",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("drain args missing %q; got: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--grace-period") {
		t.Errorf("drain args should NOT include --grace-period when GracePeriodSeconds=-1; got: %s", joined)
	}
}

func TestInvokeWrapsExecErrorWithStderr(t *testing.T) {
	c := New("/etc/k2/kubeconfig")
	c.SetRunnerForTest(&fakeRunner{
		err: map[string]error{
			"--kubeconfig=/etc/k2/kubeconfig cordon": errors.New("exit status 1"),
		},
	})
	err := c.Cordon(context.Background(), "missing-node")
	if err == nil {
		t.Fatal("expected error")
	}
	// Stderr should be folded into the error chain when c.Stderr is nil.
	if !strings.Contains(err.Error(), "exit status 1") || !strings.Contains(err.Error(), "simulated stderr") {
		t.Errorf("error %q should wrap both exec err + stderr", err)
	}
}

func TestPodsOnNodeFiltersByPhase(t *testing.T) {
	c := New("/etc/k2/kubeconfig")
	c.SetRunnerForTest(&fakeRunner{
		out: map[string]string{
			"--kubeconfig=/etc/k2/kubeconfig get pods --all-namespaces": `{
                "items": [
                  {"metadata": {"name": "a", "namespace": "x"}, "status": {"phase": "Running"}},
                  {"metadata": {"name": "b", "namespace": "x"}, "status": {"phase": "Pending"}},
                  {"metadata": {"name": "c", "namespace": "y"}, "status": {"phase": "Failed"}},
                  {"metadata": {"name": "d", "namespace": "y"}, "status": {"phase": "Succeeded"}}
                ]
            }`,
		},
	})
	bad, err := c.PodsOnNode(context.Background(), "k2-pi-335e", []string{"Running", "Succeeded"})
	if err != nil {
		t.Fatalf("PodsOnNode: %v", err)
	}
	if len(bad) != 2 {
		t.Fatalf("got %d bad pods, want 2", len(bad))
	}
	names := []string{bad[0].Name, bad[1].Name}
	if names[0] != "b" || names[1] != "c" {
		t.Errorf("got %v, want [b c]", names)
	}
}

func TestAvailableChecksPATH(t *testing.T) {
	c := New("/etc/k2/kubeconfig")
	// Default binary "kubectl" — may or may not be on PATH; just exercise
	// the code path. We can't reliably assert either way without
	// manipulating PATH, which would pollute other tests.
	_ = c.Available()

	// With a definitely-not-on-PATH binary, expect error.
	c.Binary = "/nonexistent/path/that/cannot/be/on/disk/kubectl-xyz123"
	if err := c.Available(); err == nil {
		t.Error("expected Available to fail for nonexistent binary")
	}
}

func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
