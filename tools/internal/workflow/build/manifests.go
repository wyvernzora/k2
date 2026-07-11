package build

import (
	"context"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildManifestsCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	opts := buildOptions(ctx)
	var appInfos []stepbuild.AppInfo
	var jobs []ui.JobSpec
	wf := ui.NewWorkflow(currentReporter())
	wf.Task("Prepare deploy directory", func(context.Context) error { return stepbuild.PrepareDeployDirectory(opts.RepoRoot) })
	wf.Task("Discover apps", func(context.Context) error {
		var err error
		appInfos, jobs, err = stepbuild.SynthesisJobs(opts.RepoRoot)
		return err
	})
	wf.JobGroup("Synthesize apps", func() []ui.JobSpec { return jobs }, ui.JobGroupOptions{Concurrency: opts.Jobs})
	wf.Task("Synthesize root app", func(execCtx context.Context) error {
		return stepbuild.SynthesizeRootApp(execCtx, opts.RepoRoot, appInfos)
	})
	return wf.Execute(runCtx)
}
