package toolcli

import (
	"context"
	"fmt"
	"io"
	"os"

	artifactbuild "github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/artifact"
	ocibuild "github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/oci"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *imageBuildArtifactCmd) Run(ctx *runContext, parent *imageCmd) error {
	runCtx, done := buildCommandContext()
	defer done()
	planner, err := loadImagePlanner(parent.imageGlobals())
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}

	wf := ui.NewWorkflow(reporter)
	wf.Task("Build artifact "+c.Target, func(ctx context.Context) error {
		return artifactbuild.Builder{
			Context: ctx,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}.Artifact(resolved)
	})
	return wf.Execute(runCtx)
}

func (c *imageBuildOCICmd) Run(ctx *runContext, parent *imageCmd) error {
	runCtx, done := buildCommandContext()
	defer done()
	planner, err := loadImagePlanner(parent.imageGlobals())
	if err != nil {
		return err
	}

	plans, err := imagePlansForSelection(planner, c.All, c.Target, "image build oci")
	if err != nil {
		return err
	}
	opts := ocibuild.Options{
		Push:      c.Push,
		NoCache:   c.NoCache,
		CacheFrom: c.CacheFrom,
		CacheTo:   c.CacheTo,
	}
	if len(plans) == 1 {
		return ocibuild.Builder{
			Context: runCtx,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}.Image(plans[0], opts)
	}

	wf := ui.NewWorkflow(reporter)
	wf.JobGroup("Build OCI images", func() []ui.JobSpec {
		jobs := make([]ui.JobSpec, len(plans))
		for i, resolved := range plans {
			resolved := resolved
			jobs[i] = ui.JobSpec{
				Name: resolved.Target,
				Run: func(ctx context.Context, out io.Writer) error {
					return ocibuild.Builder{
						Context: ctx,
						Stdout:  out,
						Stderr:  out,
					}.Image(resolved, opts)
				},
			}
		}
		return jobs
	}, ui.JobGroupOptions{Concurrency: ctx.jobs})
	return wf.Execute(runCtx)
}

func imagePlansForSelection(planner plan.Planner, all bool, target string, command string) ([]plan.Plan, error) {
	if all {
		if target != "" {
			return nil, fmt.Errorf("%s --all does not accept a target argument", command)
		}
		return planner.BuildAllEnabled()
	}
	if target == "" {
		return nil, fmt.Errorf("%s requires a target unless --all is set", command)
	}
	resolved, err := planner.Build(target)
	if err != nil {
		return nil, err
	}
	return []plan.Plan{resolved}, nil
}
