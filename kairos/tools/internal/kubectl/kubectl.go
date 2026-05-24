// Package kubectl is a small wrapper around the local kubectl binary,
// scoped to the operations k2-tools needs against the harvested
// cluster kubeconfig at ~/.kube/k2/<inst>/kubeconfig. It exists
// because the rest of internal/ that talks to k8s does so via SSH'd
// `sudo kubectl` on a CP node, but `k2-tools upgrade` needs to
// cordon/drain/uncordon a node BEFORE the SSH session reboots it
// out from under us — i.e. from the operator's laptop, not from
// inside the cluster.
//
// Why shell out instead of importing client-go?
//   - Operators already have kubectl installed (provisioning hands
//     them a kubeconfig path; the implied next move is kubectl).
//   - client-go pulls in ~20MB of transitive deps for what here is
//     six verbs: get, cordon, drain, uncordon (delete is a stretch
//     goal). Not worth it.
//   - Drain semantics are notoriously fiddly; reusing kubectl's own
//     implementation avoids re-implementing PDB respect, grace
//     periods, daemonset filtering, etc.
//
// The seam between this package and `exec.CommandContext` is the
// unexported `runner` interface — tests pass a fake that returns
// canned bytes; production uses the real exec runner. New verbs go
// here, not at call sites; the call sites should never `exec` kubectl
// directly so the kubeconfig flag, error wrapping, and timeout
// handling stay in one place.
package kubectl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Client wraps a local kubectl binary scoped to a single kubeconfig.
// Stdout/Stderr are written into during long-running verbs (Drain,
// notably) — wire them into a ui.Workflow Shell step so the operator
// sees the eviction log inline.
//
// Logger is optional; when set, the wrapper writes one line per
// invocation summarizing what command it ran. Useful for the
// ui.Workflow scrollback when Stdout/Stderr are the same writer as
// the bubbletea-managed terminal region.
type Client struct {
	Binary     string // defaults to "kubectl" if empty
	Kubeconfig string // path to kubeconfig file; required
	Stdout     io.Writer
	Stderr     io.Writer
	Logger     func(format string, args ...any)

	run runner // overridable for tests; nil → exec runner
}

// New constructs a Client with the given kubeconfig path. Wire
// Stdout/Stderr/Logger on the returned struct before use.
func New(kubeconfig string) *Client {
	return &Client{Binary: "kubectl", Kubeconfig: kubeconfig}
}

// Available reports whether a kubectl binary is on PATH. Used by
// the upgrade subcommand's pre-flight to refuse with a clean hint
// instead of an opaque exec error halfway through.
func (c *Client) Available() error {
	bin := c.Binary
	if bin == "" {
		bin = "kubectl"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("kubectl binary %q not found on PATH: %w", bin, err)
	}
	return nil
}

// Cordon marks the node unschedulable. Idempotent — kubectl returns
// success even if the node is already cordoned (only the message
// differs).
func (c *Client) Cordon(ctx context.Context, node string) error {
	_, err := c.invoke(ctx, []string{"cordon", node})
	if err != nil {
		return fmt.Errorf("kubectl cordon %s: %w", node, err)
	}
	return nil
}

// Uncordon marks the node schedulable. Same idempotency story as
// Cordon.
func (c *Client) Uncordon(ctx context.Context, node string) error {
	_, err := c.invoke(ctx, []string{"uncordon", node})
	if err != nil {
		return fmt.Errorf("kubectl uncordon %s: %w", node, err)
	}
	return nil
}

// DrainOpts captures the kubectl-drain flags the upgrade subcommand
// actually exercises. Zero-value defaults match the doc-recommended
// safe-but-realistic shape: ignore daemonsets (Kairos/Cilium/kube-vip
// all DaemonSet), delete emptyDir volumes (we accept the data loss
// because the node is being rebooted regardless).
type DrainOpts struct {
	Timeout            time.Duration
	IgnoreDaemonsets   bool
	DeleteEmptyDirData bool
	Force              bool
	GracePeriodSeconds int  // -1 to pass through kubectl's default
	DisableEviction    bool // skip API eviction; legacy --delete-local-data style. Off by default.
}

// Drain evicts schedulable pods off `node`. Streams kubectl's own
// progress lines through c.Stdout / c.Stderr so the operator sees
// "evicting pod foo/bar" live.
func (c *Client) Drain(ctx context.Context, node string, opts DrainOpts) error {
	args := []string{"drain", node}
	if opts.IgnoreDaemonsets {
		args = append(args, "--ignore-daemonsets")
	}
	if opts.DeleteEmptyDirData {
		args = append(args, "--delete-emptydir-data")
	}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.DisableEviction {
		args = append(args, "--disable-eviction")
	}
	if opts.Timeout > 0 {
		args = append(args, "--timeout="+opts.Timeout.String())
	}
	if opts.GracePeriodSeconds >= 0 {
		args = append(args, fmt.Sprintf("--grace-period=%d", opts.GracePeriodSeconds))
	}
	// Drain streams to the same writer as everything else; we don't
	// capture stdout, just let it flow through.
	if err := c.stream(ctx, args); err != nil {
		return fmt.Errorf("kubectl drain %s: %w", node, err)
	}
	return nil
}

// NodeAddress captures the subset of v1.NodeAddress the upgrade
// subcommand needs: type + address. Mirrors the kube API shape so
// JSON unmarshal is direct.
type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

// NodeCondition mirrors v1.NodeCondition; we only consume Type +
// Status (Ready=True/False).
type NodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Node is the slimmed shape of a v1.Node the upgrade subcommand
// consumes: the name, all addresses (so we can match --host against
// InternalIP / Hostname), the role labels (control-plane vs worker),
// and Ready condition. We deliberately keep this narrow — Status.Phase
// + Spec.Unschedulable could come along too but the upgrade flow only
// needs to know "is this a CP" and "is it Ready" today.
type Node struct {
	Name        string
	Labels      map[string]string
	Addresses   []NodeAddress
	Conditions  []NodeCondition
	Schedulable bool // !spec.unschedulable
}

// Ready reports whether the node's Ready condition is True.
func (n Node) Ready() bool {
	for _, c := range n.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// IsControlPlane reports whether the node carries either of K3s's
// control-plane role labels (`node-role.kubernetes.io/control-plane`
// or the legacy `node-role.kubernetes.io/master`). We tolerate both
// so the upgrade pre-flight doesn't get tripped by older clusters.
func (n Node) IsControlPlane() bool {
	for _, key := range []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
	} {
		if _, ok := n.Labels[key]; ok {
			return true
		}
	}
	return false
}

// InternalIP returns the first InternalIP address, or "" if the node
// has none. We use this to match --host against an existing Node.
func (n Node) InternalIP() string {
	for _, a := range n.Addresses {
		if a.Type == "InternalIP" {
			return a.Address
		}
	}
	return ""
}

// Nodes returns every Node in the cluster. Parses `kubectl get
// nodes -o json` — the wide form (-o wide) loses labels.
func (c *Client) Nodes(ctx context.Context) ([]Node, error) {
	data, err := c.invoke(ctx, []string{"get", "nodes", "-o", "json"})
	if err != nil {
		return nil, fmt.Errorf("kubectl get nodes: %w", err)
	}
	return parseNodes(data)
}

// FindNodeByInternalIP finds the single node whose InternalIP matches
// `ip`. Returns an error if zero or >1 nodes match — the upgrade
// subcommand uses this to refuse ambiguous --host values up front,
// before any SSH or kairos-agent work begins.
func (c *Client) FindNodeByInternalIP(ctx context.Context, ip string) (Node, error) {
	nodes, err := c.Nodes(ctx)
	if err != nil {
		return Node{}, err
	}
	var matched []Node
	for _, n := range nodes {
		if n.InternalIP() == ip {
			matched = append(matched, n)
		}
	}
	switch len(matched) {
	case 0:
		return Node{}, fmt.Errorf("no kubernetes node has InternalIP %s", ip)
	case 1:
		return matched[0], nil
	default:
		names := make([]string, len(matched))
		for i, n := range matched {
			names[i] = n.Name
		}
		return Node{}, fmt.Errorf("multiple kubernetes nodes have InternalIP %s: %s",
			ip, strings.Join(names, ", "))
	}
}

// PodsOnNode returns the pods scheduled on `node` whose phase is NOT
// in the allowed set. Used by the smoke check after upgrade —
// allowedPhases is typically {"Running", "Succeeded"}; anything else
// (Pending, Failed, Unknown) is a red flag worth surfacing.
type Pod struct {
	Namespace string
	Name      string
	Phase     string
}

// PodsOnNode returns pods scheduled on `node`. Pass the allowed
// phases set you consider "healthy"; the result is the pods NOT in
// that set.
func (c *Client) PodsOnNode(ctx context.Context, node string, allowedPhases []string) ([]Pod, error) {
	args := []string{
		"get", "pods", "--all-namespaces", "-o", "json",
		"--field-selector=spec.nodeName=" + node,
	}
	data, err := c.invoke(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("kubectl get pods --field-selector spec.nodeName=%s: %w", node, err)
	}
	all, err := parsePods(data)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(allowedPhases))
	for _, p := range allowedPhases {
		allowed[p] = true
	}
	var bad []Pod
	for _, p := range all {
		if !allowed[p.Phase] {
			bad = append(bad, p)
		}
	}
	return bad, nil
}

// ----- internals --------------------------------------------------------

// invoke runs kubectl with the given args, capturing stdout for
// programmatic use. Stderr still flows through c.Stderr.
func (c *Client) invoke(ctx context.Context, args []string) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	full := c.buildArgs(args)
	if c.Logger != nil {
		c.Logger("kubectl %s", strings.Join(full[1:], " "))
	}
	r := c.runnerOrDefault()
	var stdout, stderr bytes.Buffer
	err := r.run(ctx, full, nil, &stdout, &stderr)
	if c.Stderr != nil && stderr.Len() > 0 {
		_, _ = c.Stderr.Write(stderr.Bytes())
	}
	if err != nil {
		// Surface stderr in the error chain when caller didn't already
		// see it (Stderr nil).
		if c.Stderr == nil && stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// stream runs kubectl with stdout AND stderr piped to c.Stdout /
// c.Stderr. Used for long-running verbs (drain) where the operator
// expects live output.
func (c *Client) stream(ctx context.Context, args []string) error {
	if err := c.validate(); err != nil {
		return err
	}
	full := c.buildArgs(args)
	if c.Logger != nil {
		c.Logger("kubectl %s", strings.Join(full[1:], " "))
	}
	r := c.runnerOrDefault()
	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	return r.run(ctx, full, nil, stdout, stderr)
}

func (c *Client) validate() error {
	if c.Kubeconfig == "" {
		return errors.New("kubectl: Kubeconfig path is required")
	}
	return nil
}

func (c *Client) buildArgs(args []string) []string {
	bin := c.Binary
	if bin == "" {
		bin = "kubectl"
	}
	return append([]string{bin, "--kubeconfig=" + c.Kubeconfig}, args...)
}

func (c *Client) runnerOrDefault() runner {
	if c.run != nil {
		return c.run
	}
	return execRunner{}
}

// runner is the seam between this package and exec.Cmd. Tests assign
// a fake via SetRunnerForTest; production uses execRunner.
type runner interface {
	run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type execRunner struct{}

func (execRunner) run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// SetRunnerForTest swaps the underlying runner. Test-only escape
// hatch; production callers must not use it. The reason it's a
// method, not a global swap, is so a single misbehaving test can't
// taint the next test running against the same package.
func (c *Client) SetRunnerForTest(r runner) {
	c.run = r
}

// ----- JSON parsing -----------------------------------------------------

// nodeList / podList mirror the subset of `kubectl get -o json`
// output we consume. Kept private — callers see the higher-level
// Node / Pod structs.
type nodeList struct {
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Unschedulable bool `json:"unschedulable"`
		} `json:"spec"`
		Status struct {
			Addresses  []NodeAddress   `json:"addresses"`
			Conditions []NodeCondition `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type podList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
}

func parseNodes(data []byte) ([]Node, error) {
	var raw nodeList
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse kubectl get nodes JSON: %w", err)
	}
	out := make([]Node, len(raw.Items))
	for i, n := range raw.Items {
		out[i] = Node{
			Name:        n.Metadata.Name,
			Labels:      n.Metadata.Labels,
			Addresses:   n.Status.Addresses,
			Conditions:  n.Status.Conditions,
			Schedulable: !n.Spec.Unschedulable,
		}
	}
	return out, nil
}

func parsePods(data []byte) ([]Pod, error) {
	var raw podList
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse kubectl get pods JSON: %w", err)
	}
	out := make([]Pod, len(raw.Items))
	for i, p := range raw.Items {
		out[i] = Pod{
			Namespace: p.Metadata.Namespace,
			Name:      p.Metadata.Name,
			Phase:     p.Status.Phase,
		}
	}
	return out, nil
}
