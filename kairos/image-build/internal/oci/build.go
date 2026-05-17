package oci

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
)

type Builder struct {
	Stdout io.Writer
	Stderr io.Writer
	Runner CommandRunner
}

type Options struct {
	Push      bool
	NoCache   bool
	CacheFrom string
	CacheTo   string
}

type CommandRunner interface {
	Run(name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (b Builder) Image(resolved plan.Plan, options Options) error {
	if b.Stdout == nil {
		b.Stdout = io.Discard
	}
	if b.Stderr == nil {
		b.Stderr = io.Discard
	}
	if b.Runner == nil {
		b.Runner = ExecRunner{}
	}

	fmt.Fprintf(b.Stdout, "Building %s\n", resolved.Target)
	fmt.Fprintf(b.Stdout, "  image: %s\n", resolved.Image)
	fmt.Fprintf(b.Stdout, "  platform: %s\n", resolved.Platform)
	fmt.Fprintf(b.Stdout, "  kairos model: %s\n", resolved.KairosModel)
	fmt.Fprintf(b.Stdout, "  hardware: %s\n", resolved.Hardware)
	fmt.Fprintf(b.Stdout, "  overlays: %s\n", overlaysForLog(resolved.Overlays))

	args := buildArgs(resolved, options)
	if err := b.Runner.Run("docker", args, b.Stdout, b.Stderr); err != nil {
		return fmt.Errorf("OCI image build failed for %s: %w", resolved.Target, err)
	}
	return nil
}

func buildArgs(resolved plan.Plan, options Options) []string {
	outputMode := "--load"
	if options.Push {
		outputMode = "--push"
	}

	args := []string{
		"buildx", "build",
		"--platform", resolved.Platform,
		"--file", filepath.Join(resolved.Paths.BuildRoot, "Dockerfile"),
		"--build-arg", "BASE_IMAGE=" + resolved.Versions.BaseImage,
		"--build-arg", "KAIROS_INIT_VERSION=" + resolved.Versions.KairosInitVersion,
		"--build-arg", "MODEL=" + resolved.KairosModel,
		"--build-arg", "KUBERNETES_DISTRO=" + resolved.KubernetesDistro,
		"--build-arg", "KUBERNETES_VERSION=" + resolved.Versions.K3sVersion,
		"--build-arg", "VERSION=" + resolved.Versions.KairosVersion + "-" + resolved.Versions.ImageRevision,
		"--build-arg", "TRUSTED_BOOT=false",
		"--build-arg", "OVERLAYS=" + strings.Join(resolved.Overlays, ","),
		"--tag", resolved.Image,
	}
	if options.NoCache {
		args = append(args, "--no-cache")
	} else {
		if options.CacheFrom != "" {
			args = append(args, "--cache-from", options.CacheFrom)
		}
		if options.CacheTo != "" {
			args = append(args, "--cache-to", options.CacheTo)
		}
	}
	args = append(args, outputMode, repoRoot(resolved))
	return args
}

func repoRoot(resolved plan.Plan) string {
	if resolved.Paths.KairosRoot == "" {
		return "."
	}
	return filepath.Dir(resolved.Paths.KairosRoot)
}

func overlaysForLog(overlays []string) string {
	if len(overlays) == 0 {
		return "<none>"
	}
	return strings.Join(overlays, ",")
}
