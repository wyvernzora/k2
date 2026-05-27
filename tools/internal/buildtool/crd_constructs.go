package buildtool

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

func GenerateCRDConstructs(ctx context.Context, opts CRDConstructsOptions) error {
	var apps []string
	if opts.AppRoot != "" {
		appRoot := opts.AppRoot
		if !filepath.IsAbs(appRoot) {
			appRoot = filepath.Join(opts.RepoRoot, appRoot)
		}
		apps = []string{appRoot}
	} else {
		var err error
		apps, err = discoverCRDApps(opts.RepoRoot)
		if err != nil {
			return err
		}
	}
	wf := ui.NewWorkflow(opts.reporter())
	wf.JobGroup("Generate CRD constructs", func() []ui.JobSpec {
		jobs := make([]ui.JobSpec, len(apps))
		for i, appRoot := range apps {
			appRoot := appRoot
			outputDir, err := crdConstructOutputDir(opts.RepoRoot, appRoot, opts.OutputRoot)
			if err != nil {
				jobErr := err
				jobs[i] = ui.JobSpec{
					Name: filepath.Base(appRoot),
					Run: func(context.Context, io.Writer) error {
						return jobErr
					},
				}
				continue
			}
			jobs[i] = ui.JobSpec{
				Name: filepath.Base(appRoot),
				Run: func(execCtx context.Context, out io.Writer) error {
					return runCRDConstructImport(execCtx, opts.RepoRoot, appRoot, outputDir, out)
				},
			}
		}
		return jobs
	}, ui.JobGroupOptions{Concurrency: opts.Jobs})
	return wf.Execute(ctx)
}

func discoverCRDApps(repoRoot string) ([]string, error) {
	apps, err := discoverApps(repoRoot)
	if err != nil {
		return nil, err
	}
	out := apps[:0]
	for _, appRoot := range apps {
		if _, err := os.Stat(filepath.Join(appRoot, "crds", "crds.k8s.yaml")); err == nil {
			out = append(out, appRoot)
		}
	}
	return out, nil
}

func crdConstructOutputDir(repoRoot string, appRoot string, outputRoot string) (string, error) {
	if outputRoot == "" {
		return filepath.Join(appRoot, "crds"), nil
	}
	if !filepath.IsAbs(outputRoot) {
		outputRoot = filepath.Join(repoRoot, outputRoot)
	}
	relAppRoot, err := filepath.Rel(repoRoot, appRoot)
	if err != nil {
		return "", err
	}
	if relAppRoot == ".." || strings.HasPrefix(relAppRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relAppRoot) {
		return "", fmt.Errorf("app root %s is outside repository root %s", appRoot, repoRoot)
	}
	return filepath.Join(outputRoot, relAppRoot, "crds"), nil
}

func runCRDConstructImport(ctx context.Context, repoRoot string, appRoot string, outputDir string, out io.Writer) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	job := ui.CommandJob(filepath.Base(appRoot), ui.CommandSpec{
		Name: "cdk8s",
		Args: []string{"import", "-l", "typescript", "-o", outputDir, filepath.Join(appRoot, "crds", "crds.k8s.yaml")},
		Dir:  repoRoot,
	})
	return job.Run(ctx, out)
}
