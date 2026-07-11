package upgrade

import (
	"github.com/wyvernzora/k2/tools/internal/kubeconfig"
	"github.com/wyvernzora/k2/tools/internal/ui"
	workflowcore "github.com/wyvernzora/k2/tools/internal/workflow"
)

type Runtime = workflowcore.Runtime
type Registration = workflowcore.Registration

func currentReporter() *ui.Reporter { return workflowcore.Reporter() }

func kubeconfigPathFor(clusterName string) (string, error) { return kubeconfig.Path(clusterName) }
func logf(format string, args ...any)                      { currentReporter().Infof(format, args...) }
