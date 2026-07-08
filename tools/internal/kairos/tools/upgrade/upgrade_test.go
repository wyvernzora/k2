package upgrade

import (
	"testing"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/kubectl"
)

func TestParseImageRefHappyPath(t *testing.T) {
	in := `NAME="Kairos"
IMAGE_REPO=ghcr.io/wyvernzora/k2-kairos
IMAGE_LABEL=ubuntu-24.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-rev3
KAIROS_RELEASE=v3.6.0
`
	got := parseImageRef(in)
	want := "ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-rev3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseImageRefStripsQuotes(t *testing.T) {
	in := `IMAGE_REPO="ghcr.io/wyvernzora/k2-kairos"
IMAGE_LABEL='ubuntu-24.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev0'`
	got := parseImageRef(in)
	want := "ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseImageRefMissingFieldReturnsEmpty(t *testing.T) {
	if got := parseImageRef("IMAGE_REPO=foo\n"); got != "" {
		t.Errorf("expected empty when IMAGE_LABEL missing, got %q", got)
	}
	if got := parseImageRef("NAME=Kairos\n"); got != "" {
		t.Errorf("expected empty when both fields missing, got %q", got)
	}
}

func TestTagPrefixFromMetadata(t *testing.T) {
	meta := NodeImageMetadata{
		Target:   "ubuntu-24.04-arm64-rpi4cb-k8s",
		Arch:     "arm64",
		Hardware: "rpi4cb",
	}
	if got := tagPrefix(meta); got != "ubuntu-24.04-arm64-rpi4cb-k8s-" {
		t.Errorf("got %q", got)
	}
}

func TestQuorumImpactSafeWith2ReadyOthers(t *testing.T) {
	nodes := []kubectl.Node{
		makeCP("cp1", true),
		makeCP("cp2", true),
		makeCP("cp3", true),
	}
	msg, ok := quorumImpact(nodes, "cp1", false)
	if !ok {
		t.Errorf("expected safe, got refusal: %s", msg)
	}
	if msg == "" {
		t.Error("message should not be empty")
	}
}

func TestQuorumImpactRefusesOnlyReadyCP(t *testing.T) {
	nodes := []kubectl.Node{
		makeCP("cp1", true),
		makeCP("cp2", false),
		makeCP("cp3", false),
	}
	msg, ok := quorumImpact(nodes, "cp1", false)
	if ok {
		t.Errorf("expected refusal, got safe: %s", msg)
	}
}

func TestQuorumImpactAllowsLossWithFlag(t *testing.T) {
	nodes := []kubectl.Node{
		makeCP("cp1", true),
	}
	msg, ok := quorumImpact(nodes, "cp1", true)
	if !ok {
		t.Errorf("expected --allow-quorum-loss to permit, got refusal: %s", msg)
	}
	if msg == "" {
		t.Error("message should explain the override")
	}
}

func TestQuorumImpactIgnoresWorkers(t *testing.T) {
	nodes := []kubectl.Node{
		makeCP("cp1", true),
		makeCP("cp2", true),
		makeWorker("w1", true),
		makeWorker("w2", true),
	}
	msg, ok := quorumImpact(nodes, "cp1", false)
	if !ok {
		t.Errorf("expected safe with 1 other CP Ready, got refusal: %s", msg)
	}
	if msg == "1 of 2 CP (1 remain Ready — safe)" {
		// ok
	} else {
		// The exact wording is part of the operator UX; assert it
		// so accidental rewording is intentional, not silent.
		t.Errorf("got %q, want %q", msg, "1 of 2 CP (1 remain Ready — safe)")
	}
}

func TestPreflightRefusesWhenAlreadyOnTarget(t *testing.T) {
	plan := Plan{
		Current:  ImageRef{Ref: "ghcr.io/x:rev3"},
		Target:   ImageRef{Ref: "ghcr.io/x:rev3"},
		QuorumOK: true,
	}
	r := &Runner{}
	err := r.Preflight(plan)
	if err == nil {
		t.Fatal("expected refusal when current == target")
	}
}

func TestPreflightRefusesWhenQuorumNotOK(t *testing.T) {
	plan := Plan{
		Current:      ImageRef{Ref: "a"},
		Target:       ImageRef{Ref: "b"},
		QuorumOK:     false,
		QuorumImpact: "single CP",
	}
	r := &Runner{}
	if err := r.Preflight(plan); err == nil {
		t.Fatal("expected refusal when QuorumOK=false")
	}
}

func TestPreflightRefusesWhenNoTarget(t *testing.T) {
	plan := Plan{QuorumOK: true}
	r := &Runner{}
	if err := r.Preflight(plan); err == nil {
		t.Fatal("expected refusal when Target is empty")
	}
}

func TestPreflightAcceptsValidPlan(t *testing.T) {
	plan := Plan{
		Current:  ImageRef{Ref: "old"},
		Target:   ImageRef{Ref: "new"},
		QuorumOK: true,
	}
	r := &Runner{}
	if err := r.Preflight(plan); err != nil {
		t.Errorf("expected accept, got %v", err)
	}
}

func TestImageRefsMatchTrimsWhitespace(t *testing.T) {
	if !imageRefsMatch("foo:bar\n", "foo:bar") {
		t.Error("trailing newline should match")
	}
	if imageRefsMatch("foo:bar", "foo:baz") {
		t.Error("different refs should not match")
	}
}

func TestImageRefString(t *testing.T) {
	if got := (ImageRef{}).String(); got != "(unknown)" {
		t.Errorf("empty IRString got %q, want (unknown)", got)
	}
	if got := (ImageRef{Ref: "foo:bar", Created: time.Now()}).String(); got != "foo:bar" {
		t.Errorf("got %q, want foo:bar", got)
	}
}

// ----- helpers ----------------------------------------------------------

func makeCP(name string, ready bool) kubectl.Node {
	return kubectl.Node{
		Name: name,
		Labels: map[string]string{
			"node-role.kubernetes.io/control-plane": "true",
		},
		Conditions: []kubectl.NodeCondition{
			{Type: "Ready", Status: boolToStatus(ready)},
		},
	}
}

func makeWorker(name string, ready bool) kubectl.Node {
	return kubectl.Node{
		Name: name,
		Conditions: []kubectl.NodeCondition{
			{Type: "Ready", Status: boolToStatus(ready)},
		},
	}
}

func boolToStatus(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
