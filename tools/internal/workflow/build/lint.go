package build

import (
	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *buildLintCmd) Run(ctx *Runtime) error {
	runCtx, done := buildCommandContext()
	defer done()
	opts := buildOptions(ctx)
	if err := runCRDConstructs(runCtx, stepbuild.CRDConstructsOptions{Options: opts}); err != nil {
		return err
	}
	wf := ui.NewWorkflow(currentReporter())
	wf.Command("TypeScript typecheck", ui.CommandSpec{Name: "npx", Args: []string{"tsc", "--noEmit"}, Dir: opts.RepoRoot})
	wf.Command("ESLint rule tests", ui.CommandSpec{Name: "npm", Args: []string{"run", "test:eslint-rules"}, Dir: opts.RepoRoot})
	wf.Command("ESLint", ui.CommandSpec{Name: "npx", Args: []string{"eslint"}, Dir: opts.RepoRoot})
	return wf.Execute(runCtx)
}
