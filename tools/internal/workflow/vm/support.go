package vm

import (
	stepvm "github.com/wyvernzora/k2/tools/internal/step/vm"
	"github.com/wyvernzora/k2/tools/internal/ui"
	workflowcore "github.com/wyvernzora/k2/tools/internal/workflow"
)

type Runtime = workflowcore.Runtime
type Registration = workflowcore.Registration

func currentReporter() *ui.Reporter {
	return workflowcore.Reporter()
}

func vmRunner(ctx *Runtime) stepvm.Runner {
	return stepvm.Runner{RepoRoot: ctx.RepoRoot, Reporter: currentReporter()}
}
