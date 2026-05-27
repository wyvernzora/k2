package buildtool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

func ExtractCRDManifest(ctx context.Context, opts CRDManifestOptions) error {
	appRoot := opts.AppRoot
	if !filepath.IsAbs(appRoot) {
		appRoot = filepath.Join(opts.RepoRoot, appRoot)
	}
	wf := ui.NewWorkflow(opts.reporter())
	wf.Task("Extract CRDs for "+filepath.Base(appRoot), func(execCtx context.Context) error {
		return extractCRDManifest(execCtx, appRoot)
	})
	return wf.Execute(ctx)
}

func extractCRDManifest(ctx context.Context, appRoot string) error {
	result, err := renderChartCRDs(ctx, appRoot, chartCRDRenderOptions{})
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(result.Manifest)) == 0 {
		return fmt.Errorf("[%s] Helm chart emitted no CRDs", filepath.Base(appRoot))
	}
	crdDir := filepath.Join(appRoot, "crds")
	if err := os.MkdirAll(crdDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(crdDir, "crds.k8s.yaml"), result.Manifest, 0o644)
}
