package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

func SynthesizeRootApp(ctx context.Context, repoRoot string, appInfos []AppInfo) error {
	deployRoot := filepath.Join(repoRoot, "deploy")
	temp, err := os.CreateTemp("", "k2-root-apps-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	encoded, err := json.Marshal(appInfos)
	if err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	manifest, err := runCapture(ctx, repoRoot, "npx", "tsx", "build/cdk/synth-root.ts", temp.Name())
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(deployRoot, "app.k8s.yaml"), ensureTrailingNewline(manifest), 0o644)
}
