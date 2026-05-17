package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/kairos/image-build/internal/config"
	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
)

func TestBuilderRawArtifact(t *testing.T) {
	resolved := testPlan(t, []string{"raw"})
	runner := &fakeRunner{artifactDir: resolved.ArtifactDir}
	patcher := &fakePatcher{}

	var stdout bytes.Buffer
	err := Builder{
		Stdout:  &stdout,
		Stderr:  io.Discard,
		Runner:  runner,
		Patcher: patcher,
	}.Artifact(resolved)
	if err != nil {
		t.Fatal(err)
	}

	raw := filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".raw")
	compressed := raw + ".xz"
	requireNoFile(t, raw)
	requireFile(t, compressed)
	requireFile(t, filepath.Join(resolved.ArtifactDir, "SHA256SUMS"))
	requireFile(t, filepath.Join(resolved.ArtifactDir, "artifact-manifest.json"))

	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %d, want 2: %#v", len(runner.calls), runner.calls)
	}
	if runner.calls[0].name != "docker" {
		t.Fatalf("first command = %q, want docker", runner.calls[0].name)
	}
	if !slices.Contains(runner.calls[0].args, "--set") || !slices.Contains(runner.calls[0].args, "disk.state_size=8192") {
		t.Fatalf("docker args do not include disk.state_size=8192: %#v", runner.calls[0].args)
	}
	if runner.calls[1].name != "xz" {
		t.Fatalf("second command = %q, want xz", runner.calls[1].name)
	}
	if len(patcher.calls) != 1 {
		t.Fatalf("patcher calls = %d, want 1", len(patcher.calls))
	}
	if patcher.calls[0].rawFile != raw {
		t.Fatalf("patcher raw file = %q, want %q", patcher.calls[0].rawFile, raw)
	}
	if !reflect.DeepEqual(patcher.calls[0].plan.RawPatches, resolved.RawPatches) {
		t.Fatalf("patcher raw patches = %#v, want %#v", patcher.calls[0].plan.RawPatches, resolved.RawPatches)
	}
	if !strings.Contains(stdout.String(), "Artifacts written to "+resolved.ArtifactDir) {
		t.Fatalf("stdout missing success message: %s", stdout.String())
	}

	manifestBytes, err := os.ReadFile(filepath.Join(resolved.ArtifactDir, "artifact-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Target string `json:"target"`
		Image  string `json:"image"`
		Raw    struct {
			File      string `json:"file"`
			SHA256    string `json:"sha256"`
			SizeBytes int64  `json:"sizeBytes"`
		} `json:"raw"`
		Compressed struct {
			File      string `json:"file"`
			SHA256    string `json:"sha256"`
			SizeBytes int64  `json:"sizeBytes"`
		} `json:"compressed"`
		Patches struct {
			Raw []plan.RawPatch `json:"raw"`
		} `json:"patches"`
	}
	if err := json.Unmarshal(manifestBytes, &got); err != nil {
		t.Fatal(err)
	}
	if got.Target != resolved.Target || got.Image != resolved.Image {
		t.Fatalf("manifest target/image mismatch: %#v", got)
	}
	if got.Raw.File != filepath.Base(raw) || got.Raw.SHA256 == "" || got.Raw.SizeBytes != 3 {
		t.Fatalf("manifest raw mismatch: %#v", got.Raw)
	}
	if got.Compressed.File != filepath.Base(compressed) || got.Compressed.SHA256 == "" || got.Compressed.SizeBytes == 0 {
		t.Fatalf("manifest compressed mismatch: %#v", got.Compressed)
	}
	if !reflect.DeepEqual(got.Patches.Raw, resolved.RawPatches) {
		t.Fatalf("manifest raw patches = %#v, want %#v", got.Patches.Raw, resolved.RawPatches)
	}
}

func TestBuilderISOArtifact(t *testing.T) {
	resolved := testPlan(t, []string{"iso"})
	runner := &fakeRunner{artifactDir: resolved.ArtifactDir}

	if err := (Builder{Runner: runner, Patcher: &fakePatcher{}}).Artifact(resolved); err != nil {
		t.Fatal(err)
	}

	requireFile(t, filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".iso"))
	requireFile(t, filepath.Join(resolved.ArtifactDir, "SHA256SUMS"))
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	if runner.calls[0].name != "docker" {
		t.Fatalf("command = %q, want docker", runner.calls[0].name)
	}
	if !slices.Contains(runner.calls[0].args, "build-iso") {
		t.Fatalf("docker args do not include build-iso: %#v", runner.calls[0].args)
	}
}

func TestWriteChecksums(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "b.raw.xz"), "bravo")
	mustWrite(t, filepath.Join(dir, "a.raw"), "alpha")
	mustWrite(t, filepath.Join(dir, "ignore.txt"), "ignore")

	if err := WriteChecksums(dir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 1 {
		t.Fatalf("checksum lines = %#v, want 1 line", lines)
	}
	if !strings.HasSuffix(lines[0], "  b.raw.xz") {
		t.Fatalf("checksum line = %q, want b.raw.xz", lines[0])
	}
}

func TestBuilderPropagatesRunnerFailure(t *testing.T) {
	resolved := testPlan(t, []string{"raw"})
	runner := &fakeRunner{
		artifactDir: resolved.ArtifactDir,
		failCommand: "docker",
	}

	err := (Builder{Runner: runner}).Artifact(resolved)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "auroraboot raw build failed") {
		t.Fatalf("error = %v", err)
	}
}

type fakeRunner struct {
	artifactDir string
	failCommand string
	calls       []fakeCall
}

type fakeCall struct {
	name string
	args []string
}

func (r *fakeRunner) Run(name string, args []string, stdout io.Writer, stderr io.Writer) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if r.failCommand != "" && name == r.failCommand {
		return errors.New("boom")
	}

	switch {
	case name == "docker" && slices.Contains(args, "build-iso"):
		return os.WriteFile(filepath.Join(r.artifactDir, "auroraboot.iso"), []byte("iso"), 0o644)
	case name == "docker":
		return os.WriteFile(filepath.Join(r.artifactDir, "auroraboot.raw"), []byte("raw"), 0o644)
	case name == "xz":
		if len(args) == 0 {
			return errors.New("xz missing args")
		}
		raw := args[len(args)-1]
		data, err := os.ReadFile(raw)
		if err != nil {
			return err
		}
		return os.WriteFile(raw+".xz", append([]byte("xz:"), data...), 0o644)
	default:
		return nil
	}
}

type fakePatcher struct {
	calls []fakePatchCall
}

type fakePatchCall struct {
	rawFile string
	plan    plan.Plan
}

func (p *fakePatcher) Patch(rawFile string, resolved plan.Plan) error {
	p.calls = append(p.calls, fakePatchCall{rawFile: rawFile, plan: resolved})
	return nil
}

func testPlan(t *testing.T, artifacts []string) plan.Plan {
	t.Helper()

	root := t.TempDir()
	buildRoot := filepath.Join(root, "kairos", "image-build")
	kairosRoot := filepath.Join(root, "kairos")
	artifactDir := filepath.Join(kairosRoot, "artifacts", "target")
	overlaysDir := filepath.Join(kairosRoot, "overlays")
	mustMkdir(t, filepath.Join(buildRoot, "scripts", "artifact"))
	mustMkdir(t, filepath.Join(overlaysDir, "hardware", "rpi4cb", "raw", "COS_GRUB"))
	mustMkdir(t, artifactDir)

	stateSize := 8192
	return plan.Plan{
		Target:       "target",
		Arch:         "arm64",
		Artifacts:    artifacts,
		Image:        "ghcr.io/wyvernzora/k2-kairos:test",
		ArtifactStem: "test-artifact",
		ArtifactDir:  artifactDir,
		ArtifactOptions: plan.ArtifactOptions{
			Raw: plan.RawArtifactOptions{
				DiskStateSize: &stateSize,
			},
		},
		RawPatches: []plan.RawPatch{
			{
				Type:           "copy-to-partition",
				Overlay:        "hardware/rpi4cb",
				Source:         "raw/COS_GRUB/extraconfig.txt",
				PartitionLabel: "COS_GRUB",
				Path:           "extraconfig.txt",
			},
		},
		Versions: config.Versions{
			KairosVersion:     "v4.1.0",
			KairosInitVersion: "v0.13.0",
			AuroraBootVersion: "v0.19.4",
			K3sVersion:        "v1.36.0+k3s1",
		},
		Paths: plan.PlanPaths{
			BuildRoot:   buildRoot,
			OverlaysDir: overlaysDir,
		},
	}
}

func requireFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory", path)
	}
}

func requireNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, expected it to be removed", path)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
