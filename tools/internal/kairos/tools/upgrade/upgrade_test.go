package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/kubectl"
)

func TestTagPrefixFromMetadata(t *testing.T) {
	meta := NodeImageMetadata{
		Flavor:        "ubuntu-26.04",
		Role:          "k8s",
		Arch:          "arm64",
		Hardware:      "rpi4cb",
		KairosVersion: "v4.1.2",
	}
	if got := tagPrefix(meta); got != "ubuntu-26.04-v4.1.2-arm64-rpi4cb-k8s-" {
		t.Errorf("got %q", got)
	}
}

func TestImageRefFromMetadata(t *testing.T) {
	tests := []struct {
		name string
		meta NodeImageMetadata
		want string
	}{
		{
			name: "legacy Ubuntu 24.04 image",
			meta: NodeImageMetadata{
				Flavor:            "ubuntu",
				FlavorRelease:     "24.04",
				Variant:           "standard",
				Arch:              "amd64",
				Hardware:          "qemu",
				KubernetesDistro:  "k3s",
				KubernetesVersion: "v1.36.0+k3s1",
				KairosVersion:     "v4.1.0",
				ImageRevision:     "rev0",
			},
			want: "ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-standard-v4.1.0-amd64-qemu-k3s-v1.36.0-k3s1-rev0",
		},
		{
			name: "current Ubuntu 26.04 image",
			meta: NodeImageMetadata{
				Flavor:            "ubuntu-26.04",
				Role:              "k8s",
				Arch:              "amd64",
				Hardware:          "qemu",
				KubernetesDistro:  "k3s",
				KubernetesVersion: "v1.36.2+k3s1",
				KairosVersion:     "v4.1.2",
				ImageRevision:     "rev1",
			},
			want: "ghcr.io/wyvernzora/k2-kairos:ubuntu-26.04-v4.1.2-amd64-qemu-k8s-v1.36.2-k3s1-rev1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageRefFromMetadata("ghcr.io/wyvernzora/k2-kairos", tt.meta)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImageRefFromMetadataRejectsIncompleteIdentity(t *testing.T) {
	if got := imageRefFromMetadata("ghcr.io/wyvernzora/k2-kairos", NodeImageMetadata{}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestImageRepository(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{name: "tag", ref: "ghcr.io/wyvernzora/k2-kairos:rev1", want: "ghcr.io/wyvernzora/k2-kairos"},
		{name: "registry port", ref: "registry:5000/k2/image:rev1", want: "registry:5000/k2/image"},
		{name: "digest", ref: "ghcr.io/wyvernzora/k2-kairos@sha256:abc", want: "ghcr.io/wyvernzora/k2-kairos"},
		{name: "untagged", ref: "ghcr.io/wyvernzora/k2-kairos", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageRepository(tt.ref); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKairosUpgradeSourceUsesOCIType(t *testing.T) {
	ref := "ghcr.io/wyvernzora/k2-kairos:rev1"
	if got, want := kairosUpgradeSource(ref), "oci:"+ref; got != want {
		t.Errorf("got %q, want %q", got, want)
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
		Current: ImageRef{Ref: "old"},
		Target: ImageRef{
			Ref:                       "new",
			StateSizeBytes:            8 << 30,
			UpgradeSizeAllowanceBytes: 1536 << 20,
		},
		QuorumOK:            true,
		StateTotalBytes:     8 << 30,
		StateAvailableBytes: 3 << 30,
		RequiredFreeBytes:   (1536 << 20) + UpgradeSafetyMarginBytes,
	}
	r := &Runner{}
	if err := r.Preflight(plan); err != nil {
		t.Errorf("expected accept, got %v", err)
	}
}

func TestPreflightRefusesUndersizedStatePartition(t *testing.T) {
	plan := Plan{
		Current:         ImageRef{Ref: "old"},
		Target:          ImageRef{Ref: "new", StateSizeBytes: 8 << 30},
		QuorumOK:        true,
		StateTotalBytes: 4 << 30,
	}
	err := (&Runner{}).Preflight(plan)
	if err == nil || !strings.Contains(err.Error(), "too small") {
		t.Fatalf("error = %v, want state size refusal", err)
	}
}

func TestPreflightRefusesInsufficientFreeStateSpace(t *testing.T) {
	plan := Plan{
		Current: ImageRef{Ref: "old"},
		Target: ImageRef{
			Ref:                       "new",
			StateSizeBytes:            8 << 30,
			UpgradeSizeAllowanceBytes: 1536 << 20,
		},
		QuorumOK:            true,
		StateTotalBytes:     8 << 30,
		StateAvailableBytes: 2 << 30,
		RequiredFreeBytes:   (1536 << 20) + UpgradeSafetyMarginBytes,
	}
	err := (&Runner{}).Preflight(plan)
	if err == nil || !strings.Contains(err.Error(), "insufficient free space") {
		t.Fatalf("error = %v, want free space refusal", err)
	}
}

func TestParseStateCapacity(t *testing.T) {
	total, available, err := parseStateCapacity([]byte("8589934592\n6442450944\n"))
	if err != nil {
		t.Fatal(err)
	}
	if total != 8<<30 || available != 6<<30 {
		t.Fatalf("total=%d available=%d", total, available)
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

func TestBootModeProbeUsesMarkerExistence(t *testing.T) {
	tests := []struct {
		name     string
		active   bool
		recovery bool
		want     string
	}{
		{name: "active marker content is ignored", active: true, want: "active"},
		{name: "recovery marker", recovery: true, want: "recovery"},
		{name: "recovery wins if both markers exist", active: true, recovery: true, want: "recovery"},
		{name: "missing markers", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			activePath := filepath.Join(dir, "active_mode")
			recoveryPath := filepath.Join(dir, "recovery_mode")
			if tt.active {
				if err := os.WriteFile(activePath, []byte("1"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.recovery {
				if err := os.WriteFile(recoveryPath, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			out, err := exec.Command("sh", "-c", bootModeProbeScript(activePath, recoveryPath)).Output()
			if err != nil {
				t.Fatal(err)
			}
			if got := string(out); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
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
