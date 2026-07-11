package build

import (
	"context"

	stepbuild "github.com/wyvernzora/k2/tools/internal/step/build"
	"github.com/wyvernzora/k2/tools/internal/ui"
	workflowcore "github.com/wyvernzora/k2/tools/internal/workflow"
)

type Runtime = workflowcore.Runtime
type Registration = workflowcore.Registration

func currentReporter() *ui.Reporter {
	return workflowcore.Reporter()
}

func buildOptions(ctx *Runtime) stepbuild.Options {
	return stepbuild.Options{RepoRoot: ctx.RepoRoot, Jobs: ctx.Jobs}
}

func buildCommandContext() (ctx context.Context, cleanup func()) {
	ctx, cancel := context.WithCancel(context.Background())
	currentReporter().SetInterruptCancel(cancel)
	return ctx, func() {
		cancel()
		currentReporter().SetInterruptCancel(nil)
	}
}
