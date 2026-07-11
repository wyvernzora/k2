package build

import (
	"context"
	"path/filepath"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildCRDManifestCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	opts := stepbuild.CRDManifestOptions{Options: buildOptions(ctx), AppRoot: c.AppRoot}
	appRoot := opts.AppRoot
	if !filepath.IsAbs(appRoot) {
		appRoot = filepath.Join(opts.RepoRoot, appRoot)
	}
	wf := ui.NewWorkflow(currentReporter())
	wf.Task("Extract CRDs for "+filepath.Base(appRoot), func(execCtx context.Context) error {
		return stepbuild.ExtractCRDManifest(execCtx, opts)
	})
	return wf.Execute(runCtx)
}
