package build

import (
	"context"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildCRDConstructsCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	return runCRDConstructs(runCtx, stepbuild.CRDConstructsOptions{
		Options: buildOptions(ctx), OutputRoot: c.OutputRoot, AppRoot: c.AppRoot,
	})
}

func runCRDConstructs(ctx context.Context, opts stepbuild.CRDConstructsOptions) error {
	jobs, err := stepbuild.CRDConstructJobs(opts)
	if err != nil {
		return err
	}
	wf := ui.NewWorkflow(currentReporter())
	wf.JobGroup("Generate CRD constructs", func() []ui.JobSpec { return jobs }, ui.JobGroupOptions{Concurrency: opts.Jobs})
	return wf.Execute(ctx)
}
