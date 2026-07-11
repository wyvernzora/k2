package image

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/wyvernzora/k2/tools/internal/image/plan"
	stepimage "github.com/wyvernzora/k2/tools/internal/step/image"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *imageBuildOCICmd) Run(ctx *Runtime, parent *imageCmd) error {
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
	opts := stepimage.OCIOptions{
		Push:      c.Push,
		NoCache:   c.NoCache,
		CacheFrom: c.CacheFrom,
		CacheTo:   c.CacheTo,
	}
	if len(plans) == 1 {
		return stepimage.OCIBuilder{
			Context: runCtx,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}.Image(plans[0], opts)
	}

	wf := ui.NewWorkflow(currentReporter())
	wf.JobGroup("Build OCI images", func() []ui.JobSpec {
		jobs := make([]ui.JobSpec, len(plans))
		for i, resolved := range plans {
			resolved := resolved
			jobs[i] = ui.JobSpec{
				Name: resolved.Target,
				Run: func(ctx context.Context, out io.Writer) error {
					return stepimage.OCIBuilder{
						Context: ctx,
						Stdout:  out,
						Stderr:  out,
					}.Image(resolved, opts)
				},
			}
		}
		return jobs
	}, ui.JobGroupOptions{Concurrency: ctx.Jobs})
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
