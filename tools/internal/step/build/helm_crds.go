package build

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type chartCRDRenderOptions struct {
	AllowTemplateFailureWithoutStaticCRDs bool
}

type chartCRDRenderResult struct {
	Manifest       []byte
	TemplateFailed bool
	TemplateError  error
}

func renderChartCRDs(ctx context.Context, appRoot string, opts chartCRDRenderOptions) (chartCRDRenderResult, error) {
	tempDir, err := os.MkdirTemp("", "k2-crd-manifest-*")
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	defer os.RemoveAll(tempDir)

	tempAppRoot, err := prepareChartForCRDRender(appRoot, tempDir)
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	helmEnv, err := isolatedHelmEnv(tempDir)
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	chart, err := readChart(appRoot)
	if err != nil {
		return chartCRDRenderResult{}, err
	}

	if err := addHelmRepositories(ctx, helmEnv, chart.Repositories()); err != nil {
		return chartCRDRenderResult{}, err
	}
	return renderPreparedChartCRDs(ctx, filepath.Base(appRoot), tempAppRoot, helmEnv, opts)
}

func prepareChartForCRDRender(appRoot string, tempDir string) (string, error) {
	tempAppRoot := filepath.Join(tempDir, filepath.Base(appRoot))
	if err := os.CopyFS(tempAppRoot, os.DirFS(appRoot)); err != nil {
		return "", err
	}
	if err := os.RemoveAll(filepath.Join(tempAppRoot, "crds")); err != nil {
		return "", err
	}
	return tempAppRoot, nil
}

func addHelmRepositories(ctx context.Context, helmEnv []string, repositories []string) error {
	hasRepos := false
	for _, repo := range repositories {
		if strings.HasPrefix(repo, "oci://") {
			continue
		}
		hasRepos = true
		if _, err := runCaptureEnv(ctx, "", helmEnv, "helm", "repo", "add", "--force-update", helmRepoName(repo), repo); err != nil {
			return err
		}
	}
	if hasRepos {
		if _, err := runCaptureEnv(ctx, "", helmEnv, "helm", "repo", "update"); err != nil {
			return err
		}
	}
	return nil
}

func renderPreparedChartCRDs(
	ctx context.Context,
	releaseName string,
	chartRoot string,
	helmEnv []string,
	opts chartCRDRenderOptions,
) (chartCRDRenderResult, error) {
	if _, err := runCaptureEnv(ctx, "", helmEnv, "helm", "dependency", "build", chartRoot); err != nil {
		return chartCRDRenderResult{}, err
	}
	staticDocs, err := readPackagedCRDs(chartRoot)
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	result := runCommand(ctx, "", helmEnv, "helm", "template", releaseName, chartRoot, "--include-crds")
	if result.Err != nil {
		return renderChartCRDTemplateFailure(staticDocs, result.Error("render chart CRDs"), opts)
	}
	return renderChartCRDTemplateResult(result.Stdout)
}

func renderChartCRDTemplateResult(data []byte) (chartCRDRenderResult, error) {
	docs, err := parseCRDDocuments(data)
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	if len(docs) == 0 {
		return chartCRDRenderResult{}, nil
	}
	encoded, err := encodeYAMLDocuments(docs)
	if err != nil {
		return chartCRDRenderResult{}, err
	}
	return chartCRDRenderResult{Manifest: encoded}, nil
}

func renderChartCRDTemplateFailure(
	staticDocs []*yaml.Node,
	templateErr error,
	opts chartCRDRenderOptions,
) (chartCRDRenderResult, error) {
	if len(staticDocs) > 0 {
		encoded, err := encodeYAMLDocuments(staticDocs)
		if err != nil {
			return chartCRDRenderResult{}, err
		}
		return chartCRDRenderResult{Manifest: encoded, TemplateFailed: true, TemplateError: templateErr}, nil
	}
	if opts.AllowTemplateFailureWithoutStaticCRDs {
		return chartCRDRenderResult{TemplateFailed: true, TemplateError: templateErr}, nil
	}
	return chartCRDRenderResult{}, templateErr
}

func readPackagedCRDs(chartRoot string) ([]*yaml.Node, error) {
	var crds []*yaml.Node
	appendCRDs := func(data []byte) error {
		docs, err := parseCRDDocuments(data)
		if err != nil {
			return err
		}
		crds = append(crds, docs...)
		return nil
	}
	if err := filepath.WalkDir(chartRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isCRDYAMLPath(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return appendCRDs(data)
	}); err != nil {
		return nil, err
	}
	chartsDir := filepath.Join(chartRoot, "charts")
	if _, err := os.Stat(chartsDir); err != nil {
		if os.IsNotExist(err) {
			return crds, nil
		}
		return nil, err
	}
	if err := filepath.WalkDir(chartsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".tgz") {
			return nil
		}
		docs, err := readPackagedCRDsFromArchive(path)
		if err != nil {
			return err
		}
		crds = append(crds, docs...)
		return nil
	}); err != nil {
		return nil, err
	}
	return crds, nil
}

func readPackagedCRDsFromArchive(path string) ([]*yaml.Node, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var crds []*yaml.Node
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || !isCRDYAMLPath(header.Name) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		docs, err := parseCRDDocuments(data)
		if err != nil {
			return nil, err
		}
		crds = append(crds, docs...)
	}
	return crds, nil
}

func isCRDYAMLPath(path string) bool {
	slashPath := filepath.ToSlash(path)
	if !strings.HasSuffix(slashPath, ".yaml") && !strings.HasSuffix(slashPath, ".yml") {
		return false
	}
	parts := strings.Split(slashPath, "/")
	for _, part := range parts {
		if part == "templates" {
			return false
		}
	}
	for _, part := range parts {
		if part != "crds" {
			continue
		}
		return true
	}
	return false
}

func helmRepoName(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return "repo-" + hex.EncodeToString(sum[:6])
}

func isolatedHelmEnv(root string) ([]string, error) {
	cache := filepath.Join(root, "helm-cache")
	registryDir := filepath.Join(root, "helm-registry")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		return nil, err
	}
	return append(os.Environ(),
		"HELM_REPOSITORY_CONFIG="+filepath.Join(root, "repositories.yaml"),
		"HELM_REPOSITORY_CACHE="+cache,
		"HELM_REGISTRY_CONFIG="+filepath.Join(registryDir, "config.json"),
	), nil
}
