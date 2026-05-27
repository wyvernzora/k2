package buildtool

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCRDConstructOutputDirDefaultsToAppCRDs(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	appRoot := filepath.Join(repoRoot, "apps", "demo")

	got, err := crdConstructOutputDir(repoRoot, appRoot, "")
	if err != nil {
		t.Fatalf("crdConstructOutputDir: %v", err)
	}
	want := filepath.Join(appRoot, "crds")
	if got != want {
		t.Fatalf("output dir = %q, want %q", got, want)
	}
}

func TestCRDConstructOutputDirMirrorsRepoPathUnderOutputRoot(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	appRoot := filepath.Join(repoRoot, "apps", "demo")
	outputRoot := filepath.Join(t.TempDir(), "out")

	got, err := crdConstructOutputDir(repoRoot, appRoot, outputRoot)
	if err != nil {
		t.Fatalf("crdConstructOutputDir: %v", err)
	}
	want := filepath.Join(outputRoot, "apps", "demo", "crds")
	if got != want {
		t.Fatalf("output dir = %q, want %q", got, want)
	}
}

func TestCRDConstructOutputDirRejectsAppOutsideRepo(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	appRoot := filepath.Join(t.TempDir(), "apps", "demo")

	if _, err := crdConstructOutputDir(repoRoot, appRoot, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatalf("expected app outside repository to fail")
	}
}

func TestStripSynthesizedCRDsStripsCommittedCRDs(t *testing.T) {
	appRoot, manifestPath := writeCRDDriftFixture(t, crdFixture("v1", "old"), crdFixture("v1", "old"))

	if err := stripSynthesizedCRDs(appRoot, manifestPath, true, &bytes.Buffer{}); err != nil {
		t.Fatalf("stripSynthesizedCRDs: %v", err)
	}
	got := mustRead(t, manifestPath)
	if strings.Contains(got, "CustomResourceDefinition") {
		t.Fatalf("manifest still contains CRD:\n%s", got)
	}
	if !strings.Contains(got, "kind: ConfigMap") {
		t.Fatalf("manifest lost non-CRD document:\n%s", got)
	}
}

func TestValidateRenderedCRDsFailsOnCRDDriftWithoutManifestCRDs(t *testing.T) {
	appRoot := writeCommittedCRDFixture(t, crdFixture("v1", "old"))

	err := validateRenderedCRDs(appRoot, []byte(crdFixture("v2", "old")), nil)
	if err == nil {
		t.Fatalf("expected CRD drift error")
	}
	if !strings.Contains(err.Error(), "Helm chart CRDs differ") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRenderedCRDsHonorsIgnoredCRDFields(t *testing.T) {
	appRoot := writeCommittedCRDFixture(t, crdFixture("v1", "old"))

	if err := validateRenderedCRDs(appRoot, []byte(crdFixture("v1", "new")), []string{"/metadata/labels/ignored"}); err != nil {
		t.Fatalf("validateRenderedCRDs: %v", err)
	}
}

func TestParseGitNameStatusHandlesRenames(t *testing.T) {
	remoteRoot := filepath.Join("tmp", "remote")
	deployRoot := filepath.Join("tmp", "deploy")
	data := []byte("R100\x00" + filepath.Join(remoteRoot, "old.yaml") + "\x00" + filepath.Join(deployRoot, "new.yaml") + "\x00")

	got, err := parseGitNameStatus(data, remoteRoot, deployRoot)
	if err != nil {
		t.Fatalf("parseGitNameStatus: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("entries = %d, want 1", len(got))
	}
	if got[0].Status != "R100" || got[0].Path1 != "old.yaml" || got[0].Path2 != "new.yaml" {
		t.Fatalf("unexpected rename entry: %#v", got[0])
	}
}

func TestWriteDiffEntryRejectsUnsupportedStatus(t *testing.T) {
	err := writeDiffEntry(&bytes.Buffer{}, diffEntry{Status: "T", Path1: "app.k8s.yaml"}, t.TempDir(), t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported git diff status") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseCRDDocumentsUsesStructuredKind(t *testing.T) {
	docs, err := parseCRDDocuments([]byte("apiVersion: apiextensions.k8s.io/v1\nkind: \"CustomResourceDefinition\"\nmetadata:\n  name: widgets.example.com\nspec:\n  note: |\n    ---\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"))
	if err != nil {
		t.Fatalf("parseCRDDocuments: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	if got := yamlMetadataName(docs[0]); got != "widgets.example.com" {
		t.Fatalf("metadata.name = %s", got)
	}
}

func TestChartYAMLAllowCRDTemplateFailureIsExplicit(t *testing.T) {
	chart := chartYAML{}
	if chart.AllowCRDTemplateFailure() {
		t.Fatalf("missing annotation should not allow CRD template failure")
	}
	chart.Annotations = map[string]string{allowCRDTemplateFailureAnnotation: "true"}
	if !chart.AllowCRDTemplateFailure() {
		t.Fatalf("explicit true annotation should allow CRD template failure")
	}
}

func TestChartYAMLAllowCRDEmptyRenderIsExplicit(t *testing.T) {
	chart := chartYAML{}
	if chart.AllowCRDEmptyRender() {
		t.Fatalf("missing annotation should not allow empty CRD render")
	}
	chart.Annotations = map[string]string{allowCRDEmptyRenderAnnotation: "true"}
	if !chart.AllowCRDEmptyRender() {
		t.Fatalf("explicit true annotation should allow empty CRD render")
	}
}

func TestValidateChartCRDRenderRejectsCommittedCRDsWithoutRenderedCRDs(t *testing.T) {
	appRoot := writeCommittedCRDFixture(t, crdFixture("v1", "old"))

	err := validateChartCRDRender(appRoot, chartYAML{}, chartCRDRenderResult{}, true, nil, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected empty CRD render error")
	}
	if !strings.Contains(err.Error(), "CRD drift cannot be checked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateChartCRDRenderAllowsAnnotatedCommittedCRDsWithoutRenderedCRDs(t *testing.T) {
	appRoot := writeCommittedCRDFixture(t, crdFixture("v1", "old"))
	chart := chartYAML{Annotations: map[string]string{allowCRDEmptyRenderAnnotation: "true"}}
	var out bytes.Buffer

	if err := validateChartCRDRender(appRoot, chart, chartCRDRenderResult{}, true, nil, &out); err != nil {
		t.Fatalf("validateChartCRDRender: %v", err)
	}
	if !strings.Contains(out.String(), allowCRDEmptyRenderAnnotation) {
		t.Fatalf("warning did not mention explicit annotation:\n%s", out.String())
	}
}

func writeCRDDriftFixture(t *testing.T, committedCRD string, renderedCRD string) (appRoot string, manifestPath string) {
	t.Helper()
	appRoot = writeCommittedCRDFixture(t, committedCRD)
	manifestPath = filepath.Join(filepath.Dir(filepath.Dir(appRoot)), "deploy", "demo", "app.k8s.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := strings.TrimSpace(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
`) + "\n---\n" + renderedCRD
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return appRoot, manifestPath
}

func writeCommittedCRDFixture(t *testing.T, committedCRD string) string {
	t.Helper()
	dir := t.TempDir()
	appRoot := filepath.Join(dir, "apps", "demo")
	crdDir := filepath.Join(appRoot, "crds")
	if err := os.MkdirAll(crdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crdDir, "crds.k8s.yaml"), []byte(committedCRD), 0o644); err != nil {
		t.Fatal(err)
	}
	return appRoot
}

func crdFixture(version string, ignoredLabel string) string {
	return `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
  labels:
    ignored: ` + ignoredLabel + `
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
    singular: widget
  scope: Namespaced
  versions:
    - name: ` + version + `
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
