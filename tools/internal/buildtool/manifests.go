package buildtool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wyvernzora/k2/tools/internal/buildtool/dyffignore"
	"github.com/wyvernzora/k2/tools/internal/ui"
	"gopkg.in/yaml.v3"
)

func SynthesizeManifests(ctx context.Context, opts Options) error {
	wf := ui.NewWorkflow(opts.reporter())
	var (
		appInfos []appInfo
		jobSpecs []ui.JobSpec
	)
	wf.Task("Prepare deploy directory", func(execCtx context.Context) error {
		deployRoot := filepath.Join(opts.RepoRoot, "deploy")
		if err := os.RemoveAll(deployRoot); err != nil {
			return err
		}
		return os.MkdirAll(deployRoot, 0o755)
	})
	wf.Task("Discover apps", func(execCtx context.Context) error {
		var err error
		appInfos, jobSpecs, err = synthAppJobs(opts.RepoRoot)
		return err
	})
	wf.JobGroup("Synthesize apps", func() []ui.JobSpec {
		return jobSpecs
	}, ui.JobGroupOptions{Concurrency: opts.Jobs})
	wf.Task("Synthesize root app", func(execCtx context.Context) error {
		return synthesizeRootApp(execCtx, opts.RepoRoot, appInfos, filepath.Join(opts.RepoRoot, "deploy"))
	})
	return wf.Execute(ctx)
}

func synthAppJobs(repoRoot string) ([]appInfo, []ui.JobSpec, error) {
	appRoots, err := discoverApps(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	rules, err := dyffignore.Load(filepath.Join(repoRoot, dyffIgnorePath))
	if err != nil {
		return nil, nil, err
	}
	appInfos := make([]appInfo, len(appRoots))
	jobSpecs := make([]ui.JobSpec, len(appRoots))
	for i, appRoot := range appRoots {
		appName := filepath.Base(appRoot)
		appInfos[i] = makeAppInfo(appName)
		jobSpecs[i] = ui.JobSpec{
			Name: appName,
			Run: func(ctx context.Context, out io.Writer) error {
				return synthesizeApp(ctx, repoRoot, appRoot, rules.Section("crd"), out)
			},
		}
	}
	return appInfos, jobSpecs, nil
}

func synthesizeApp(ctx context.Context, repoRoot string, appRoot string, crdExcludes []string, out io.Writer) error {
	appName := filepath.Base(appRoot)
	manifest, err := runCapture(ctx, repoRoot, "npx", "tsx", "build/cdk/synth-app.ts", filepath.ToSlash(appRoot))
	if err != nil {
		return err
	}
	appOutDir := filepath.Join(repoRoot, "deploy", appName)
	if err := os.MkdirAll(appOutDir, 0o755); err != nil {
		return err
	}
	appManifest := filepath.Join(appOutDir, "app.k8s.yaml")
	if err := os.WriteFile(appManifest, ensureTrailingNewline(manifest), 0o644); err != nil {
		return err
	}
	hasCommittedCRDs, err := copyCommittedCRDs(appRoot, appOutDir, out)
	if err != nil {
		return err
	}
	if chart, ok, err := readOptionalChart(appRoot); err != nil {
		return err
	} else if ok {
		renderResult, err := renderChartCRDs(ctx, appRoot, chartCRDRenderOptions{
			AllowTemplateFailureWithoutStaticCRDs: !hasCommittedCRDs && chart.AllowCRDTemplateFailure(),
		})
		if err != nil {
			return err
		}
		if err := validateChartCRDRender(appRoot, chart, renderResult, hasCommittedCRDs, crdExcludes, out); err != nil {
			return err
		}
	}
	if err := stripSynthesizedCRDs(appRoot, appManifest, hasCommittedCRDs, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "synthesized %s\n", appName)
	return nil
}

func readOptionalChart(appRoot string) (chartYAML, bool, error) {
	if !chartExists(appRoot) {
		return chartYAML{}, false, nil
	}
	chart, err := readChart(appRoot)
	if err != nil {
		return chartYAML{}, false, err
	}
	return chart, true, nil
}

func validateChartCRDRender(appRoot string, chart chartYAML, renderResult chartCRDRenderResult, hasCommittedCRDs bool, crdExcludes []string, out io.Writer) error {
	appName := filepath.Base(appRoot)
	if renderResult.TemplateFailed && !hasCommittedCRDs && len(bytes.TrimSpace(renderResult.Manifest)) == 0 {
		jobWarnf(out, "[%s] skipped templated CRD probe because Helm template failed and the chart has no packaged CRDs", appName)
	}
	if len(bytes.TrimSpace(renderResult.Manifest)) > 0 {
		if !hasCommittedCRDs {
			return fmt.Errorf("[%s] Helm chart emitted CRDs, but apps/%s/crds/crds.k8s.yaml is missing\nRun: earthly +crd-manifest --APP_ROOT=apps/%s", appName, appName, appName)
		}
		return validateRenderedCRDs(appRoot, renderResult.Manifest, crdExcludes)
	}
	if !hasCommittedCRDs {
		return nil
	}
	if chart.AllowCRDEmptyRender() {
		jobWarnf(out, "[%s] chart emitted no CRDs; copied committed CRDs without drift check due to %s=true", appName, allowCRDEmptyRenderAnnotation)
		return nil
	}
	return fmt.Errorf("[%s] apps/%s/crds/crds.k8s.yaml exists, but Helm chart emitted no CRDs\nRefusing to synthesize manifests because CRD drift cannot be checked.\nIf these CRDs are intentionally sourced outside the chart, add annotation %s: \"true\" to apps/%s/Chart.yaml with an explanatory comment", appName, appName, allowCRDEmptyRenderAnnotation, appName)
}

func synthesizeRootApp(ctx context.Context, repoRoot string, appInfos []appInfo, deployRoot string) error {
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

func copyCommittedCRDs(appRoot string, outdir string, out io.Writer) (bool, error) {
	src := filepath.Join(appRoot, "crds", "crds.k8s.yaml")
	data, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(filepath.Join(appRoot, "crds")); statErr == nil {
				jobWarnf(out, "[%s] crds/ exists but has no crds.k8s.yaml", filepath.Base(appRoot))
			}
			return false, nil
		}
		return false, err
	}
	return true, os.WriteFile(filepath.Join(outdir, "crds.k8s.yaml"), data, 0o644)
}

func stripSynthesizedCRDs(appRoot string, manifestPath string, hasCommittedCRDs bool, out io.Writer) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	docs, err := parseYAMLDocuments(data)
	if err != nil {
		return err
	}
	var kept []*yaml.Node
	var crds []*yaml.Node
	var crdNames []string
	for _, doc := range docs {
		if yamlKind(doc) == "CustomResourceDefinition" {
			crds = append(crds, doc)
			crdNames = append(crdNames, yamlMetadataName(doc))
			continue
		}
		kept = append(kept, doc)
	}
	if len(crds) == 0 {
		return nil
	}
	appName := filepath.Base(appRoot)
	if !hasCommittedCRDs {
		return fmt.Errorf("[%s] synthesized %d CRD(s) into app.k8s.yaml, but apps/%s/crds/crds.k8s.yaml is missing", appName, len(crds), appName)
	}
	stripped, err := encodeYAMLDocuments(kept)
	if err != nil {
		return err
	}
	jobWarnf(out, "[%s] stripped %d CRD(s) from app.k8s.yaml; using crds.k8s.yaml instead", appName, len(crdNames))
	return os.WriteFile(manifestPath, stripped, 0o644)
}

func jobWarnf(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, "warning: "+format+"\n", args...)
}
