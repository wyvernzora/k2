package e2e

import (
	"context"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/kubeconfig"
	"github.com/wyvernzora/k2/tools/internal/ui"
	workflowcore "github.com/wyvernzora/k2/tools/internal/workflow"
	workflowprovision "github.com/wyvernzora/k2/tools/internal/workflow/provision"
)

type Runtime = workflowcore.Runtime
type Registration = workflowcore.Registration
type storageCredentials = workflowprovision.StorageCredentials

func currentReporter() *ui.Reporter { return workflowcore.Reporter() }

func clusterCredentialsDir(clusterName string) (string, error) {
	return kubeconfig.CredentialsDir(clusterName)
}
func kubeconfigPathFor(clusterName string) (string, error) { return kubeconfig.Path(clusterName) }

func sleepCtx(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
