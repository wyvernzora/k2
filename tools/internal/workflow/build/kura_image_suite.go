package build

import (
	"context"
	"fmt"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildKuraImageSuiteCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	wf := ui.NewWorkflow(currentReporter())
	wf.Shell("Check Kura image suite", func(execCtx context.Context, sh ui.Step) error {
		revision, err := stepbuild.CheckKuraImageSuite(execCtx, ctx.RepoRoot)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(sh, "Kura image suite source revision: %s\n", revision)
		return err
	})
	return wf.Execute(runCtx)
}
