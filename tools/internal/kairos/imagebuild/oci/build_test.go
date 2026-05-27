package oci

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/config"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
)

func TestBuilderImage(t *testing.T) {
	resolved := testPlan()
	runner := &fakeRunner{}

	var stdout bytes.Buffer
	err := Builder{
		Stdout: &stdout,
		Stderr: io.Discard,
		Runner: runner,
	}.Image(resolved, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1: %#v", len(runner.calls), runner.calls)
	}
	call := runner.calls[0]
	if call.name != "docker" {
		t.Fatalf("command = %q, want docker", call.name)
	}

	wantArgs := []string{
		"buildx", "build",
		"--platform", "linux/arm64",
		"--file", filepath.Join(string(filepath.Separator), "repo", "kairos", "Dockerfile"),
		"--build-arg", "BASE_IMAGE=ubuntu:24.04",
		"--build-arg", "KAIROS_INIT_VERSION=v0.13.0",
		"--build-arg", "MODEL=rpi4",
		"--build-arg", "KUBERNETES_DISTRO=k3s",
		"--build-arg", "KUBERNETES_VERSION=v1.36.0+k3s1",
		"--build-arg", "KAIROS_VERSION=v4.1.0",
		"--build-arg", "VERSION=v4.1.0-rev0",
		"--build-arg", "IMAGE_REVISION=rev0",
		"--build-arg", "TRUSTED_BOOT=false",
		"--build-arg", "OVERLAYS=hardware/rpi4cb",
		"--build-arg", "TARGET_NAME=ubuntu-24.04-standard-arm64-rpi4cb-k3s",
		"--build-arg", "FLAVOR=ubuntu",
		"--build-arg", "FLAVOR_RELEASE=24.04",
		"--build-arg", "VARIANT=standard",
		"--build-arg", "ARCH=arm64",
		"--build-arg", "PLATFORM=linux/arm64",
		"--build-arg", "HARDWARE=rpi4cb",
		"--tag", "ghcr.io/wyvernzora/k2-kairos:test",
		"--load",
		"/repo",
	}
	if !slices.Equal(call.args, wantArgs) {
		t.Fatalf("docker args:\n got: %#v\nwant: %#v", call.args, wantArgs)
	}

	out := stdout.String()
	for _, want := range []string{
		"Building ubuntu-24.04-standard-arm64-rpi4cb-k3s",
		"image: ghcr.io/wyvernzora/k2-kairos:test",
		"platform: linux/arm64",
		"kairos model: rpi4",
		"hardware: rpi4cb",
		"overlays: hardware/rpi4cb",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestBuilderImagePushNoCache(t *testing.T) {
	resolved := testPlan()
	runner := &fakeRunner{}

	err := (Builder{Runner: runner}).Image(resolved, Options{
		Push:    true,
		NoCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	args := runner.calls[0].args
	if !slices.Contains(args, "--no-cache") {
		t.Fatalf("docker args missing --no-cache: %#v", args)
	}
	if !slices.Contains(args, "--push") {
		t.Fatalf("docker args missing --push: %#v", args)
	}
	if slices.Contains(args, "--load") {
		t.Fatalf("docker args unexpectedly include --load: %#v", args)
	}
}

func TestBuilderImageCacheOptions(t *testing.T) {
	resolved := testPlan()
	runner := &fakeRunner{}

	err := (Builder{Runner: runner}).Image(resolved, Options{
		CacheFrom: "type=local,src=/cache/current",
		CacheTo:   "type=local,dest=/cache/next,mode=max",
	})
	if err != nil {
		t.Fatal(err)
	}

	args := runner.calls[0].args
	cacheFrom := indexOf(args, "--cache-from")
	if cacheFrom == -1 || cacheFrom+1 >= len(args) || args[cacheFrom+1] != "type=local,src=/cache/current" {
		t.Fatalf("docker args missing cache-from pair: %#v", args)
	}
	cacheTo := indexOf(args, "--cache-to")
	if cacheTo == -1 || cacheTo+1 >= len(args) || args[cacheTo+1] != "type=local,dest=/cache/next,mode=max" {
		t.Fatalf("docker args missing cache-to pair: %#v", args)
	}
}

func TestBuilderImageNoCacheSkipsCacheOptions(t *testing.T) {
	resolved := testPlan()
	runner := &fakeRunner{}

	err := (Builder{Runner: runner}).Image(resolved, Options{
		NoCache:   true,
		CacheFrom: "type=local,src=/cache/current",
		CacheTo:   "type=local,dest=/cache/next,mode=max",
	})
	if err != nil {
		t.Fatal(err)
	}

	args := runner.calls[0].args
	if slices.Contains(args, "--cache-from") || slices.Contains(args, "--cache-to") {
		t.Fatalf("docker args unexpectedly include cache flags with --no-cache: %#v", args)
	}
}

func TestBuilderImagePropagatesRunnerFailure(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}

	err := (Builder{Runner: runner}).Image(testPlan(), Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "OCI image build failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuilderImagePassesContextToRunner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &fakeRunner{}

	if err := (Builder{Context: ctx, Runner: runner}).Image(testPlan(), Options{}); err != nil {
		t.Fatal(err)
	}
	if len(runner.ctxErrs) != 1 || !errors.Is(runner.ctxErrs[0], context.Canceled) {
		t.Fatalf("runner context errors = %#v, want context.Canceled", runner.ctxErrs)
	}
}

type fakeRunner struct {
	err     error
	calls   []fakeCall
	ctxErrs []error
}

type fakeCall struct {
	name string
	args []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	r.ctxErrs = append(r.ctxErrs, ctx.Err())
	return r.err
}

func indexOf(values []string, needle string) int {
	for i, value := range values {
		if value == needle {
			return i
		}
	}
	return -1
}

func testPlan() plan.Plan {
	return plan.Plan{
		Target:           "ubuntu-24.04-standard-arm64-rpi4cb-k3s",
		Flavor:           "ubuntu",
		FlavorRelease:    "24.04",
		Variant:          "standard",
		Arch:             "arm64",
		Platform:         "linux/arm64",
		KairosModel:      "rpi4",
		Hardware:         "rpi4cb",
		KubernetesDistro: "k3s",
		Overlays:         []string{"hardware/rpi4cb"},
		Image:            "ghcr.io/wyvernzora/k2-kairos:test",
		Versions: config.Versions{
			KairosVersion:     "v4.1.0",
			KairosInitVersion: "v0.13.0",
			BaseImage:         "ubuntu:24.04",
			K3sVersion:        "v1.36.0+k3s1",
			ImageRevision:     "rev0",
		},
		Paths: plan.PlanPaths{
			BuildRoot:  "/repo/kairos",
			KairosRoot: "/repo/kairos",
		},
	}
}
