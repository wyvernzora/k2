package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func ExtractCRDManifest(ctx context.Context, opts CRDManifestOptions) error {
	appRoot := opts.AppRoot
	if !filepath.IsAbs(appRoot) {
		appRoot = filepath.Join(opts.RepoRoot, appRoot)
	}
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
