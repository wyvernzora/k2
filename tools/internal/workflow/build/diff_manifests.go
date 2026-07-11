package build

import (
	"context"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildDiffManifestsCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	opts := stepbuild.DiffManifestsOptions{Options: buildOptions(ctx), RemoteURL: c.RemoteURL}
	wf := ui.NewWorkflow(currentReporter())
	wf.Task("Diff deploy manifests", func(execCtx context.Context) error { return stepbuild.DiffManifests(execCtx, opts) })
	return wf.Execute(runCtx)
}
