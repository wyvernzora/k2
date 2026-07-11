package dyffx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBetweenFilesRespectsExcludes(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from.yaml")
	to := filepath.Join(dir, "to.yaml")
	mustWrite(t, from, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  labels:\n    ignored: old\ndata:\n  keep: old\n")
	mustWrite(t, to, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  labels:\n    ignored: new\ndata:\n  keep: new\n")

	out, different, err := BetweenFiles(from, to, []string{"/metadata/labels/ignored"})
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if !different {
		t.Fatalf("expected data diff")
	}
	if strings.Contains(out, "ignored") {
		t.Fatalf("excluded label appeared in diff:\n%s", out)
	}
	if !strings.Contains(out, "keep") {
		t.Fatalf("expected kept data diff:\n%s", out)
	}
}

func TestBetweenFilesExcludesSubtreesBeforeEntityMatching(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from.yaml")
	to := filepath.Join(dir, "to.yaml")
	mustWrite(t, from, "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: demos.example.com\n  namespace: k2-core\n  labels:\n    helm.sh/chart: old\nspec:\n  group: example.com\n")
	mustWrite(t, to, "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: demos.example.com\n  labels:\n    helm.sh/chart: new\nspec:\n  group: example.com\n")

	out, different, err := BetweenFiles(from, to, []string{"/metadata/namespace", "/metadata/labels"})
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if different {
		t.Fatalf("expected namespace and label subtree to be ignored, got diff:\n%s", out)
	}
}

func TestBetweenFilesExcludesEscapedSlashKeys(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from.yaml")
	to := filepath.Join(dir, "to.yaml")
	mustWrite(t, from, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  labels:\n    helm.sh/chart: old\n    keep: old\n")
	mustWrite(t, to, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  labels:\n    helm.sh/chart: new\n    keep: new\n")

	out, different, err := BetweenFiles(from, to, []string{"/metadata/labels/helm.sh\\/chart"})
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if !different {
		t.Fatalf("expected keep label diff")
	}
	if strings.Contains(out, "helm.sh/chart") {
		t.Fatalf("escaped slash label appeared in diff:\n%s", out)
	}
	if !strings.Contains(out, "keep") {
		t.Fatalf("expected kept label diff:\n%s", out)
	}
}

func TestBetweenFilesReportsOrderChanges(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from.yaml")
	to := filepath.Join(dir, "to.yaml")
	mustWrite(t, from, "items:\n- a\n- b\n")
	mustWrite(t, to, "items:\n- b\n- a\n")

	out, different, err := BetweenFiles(from, to, nil)
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if !different {
		t.Fatalf("expected order diff")
	}
	if !strings.Contains(out, "items") {
		t.Fatalf("expected items in order diff:\n%s", out)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
