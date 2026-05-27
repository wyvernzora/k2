package buildtool

import (
	"context"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

func Lint(ctx context.Context, opts Options) error {
	if err := GenerateCRDConstructs(ctx, CRDConstructsOptions{Options: opts}); err != nil {
		return err
	}
	wf := ui.NewWorkflow(opts.reporter())
	wf.Command("TypeScript typecheck", ui.CommandSpec{Name: "npx", Args: []string{"tsc", "--noEmit"}, Dir: opts.RepoRoot})
	wf.Command("ESLint rule tests", ui.CommandSpec{Name: "npm", Args: []string{"run", "test:eslint-rules"}, Dir: opts.RepoRoot})
	wf.Command("ESLint", ui.CommandSpec{Name: "npx", Args: []string{"eslint"}, Dir: opts.RepoRoot})
	return wf.Execute(ctx)
}
