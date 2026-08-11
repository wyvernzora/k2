package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKuraRevision = "6e7e7519847dbbde790fc5d8300f5ba1cf5d7ede"

type fakeSourceRevisionResolver struct {
	revisions map[string]string
	errors    map[string]error
	seen      []string
}

func (f *fakeSourceRevisionResolver) SourceRevision(_ context.Context, ref string) (string, error) {
	f.seen = append(f.seen, ref)
	if err := f.errors[ref]; err != nil {
		return "", err
	}
	return f.revisions[ref], nil
}

func TestCheckKuraImageSuiteAcceptsCompleteConsistentTuple(t *testing.T) {
	refs := testKuraRefs()
	resolver := &fakeSourceRevisionResolver{revisions: map[string]string{
		refs["libraryManager"]: testKuraRevision,
		refs["gateway"]:        testKuraRevision,
		refs["releaseIndexer"]: testKuraRevision,
		refs["n8nNodes"]:       testKuraRevision,
	}}

	got, err := checkKuraImageSuite(t.Context(), writeKuraImagesFixture(t, refs), resolver)
	if err != nil {
		t.Fatalf("checkKuraImageSuite: %v", err)
	}
	if got != testKuraRevision {
		t.Fatalf("revision = %q, want %q", got, testKuraRevision)
	}
	if len(resolver.seen) != 4 {
		t.Fatalf("inspected refs = %d, want 4", len(resolver.seen))
	}
}

func TestCheckKuraImageSuiteRejectsIncompleteTuple(t *testing.T) {
	refs := testKuraRefs()
	delete(refs, "gateway")

	_, err := checkKuraImageSuite(
		t.Context(),
		writeKuraImagesFixture(t, refs),
		&fakeSourceRevisionResolver{},
	)
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Fatalf("error = %v, want missing gateway", err)
	}
}

func TestCheckKuraImageSuiteRejectsTagOnlyReference(t *testing.T) {
	refs := testKuraRefs()
	refs["gateway"] = "ghcr.io/wyvernzora/kura/gateway:main"

	_, err := checkKuraImageSuite(
		t.Context(),
		writeKuraImagesFixture(t, refs),
		&fakeSourceRevisionResolver{},
	)
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Fatalf("error = %v, want invalid gateway pin", err)
	}
}

func TestCheckKuraImageSuiteRejectsMixedRevisions(t *testing.T) {
	refs := testKuraRefs()
	resolver := &fakeSourceRevisionResolver{revisions: map[string]string{
		refs["libraryManager"]: testKuraRevision,
		refs["gateway"]:        testKuraRevision,
		refs["releaseIndexer"]: "0123456789abcdef0123456789abcdef01234567",
		refs["n8nNodes"]:       testKuraRevision,
	}}

	_, err := checkKuraImageSuite(t.Context(), writeKuraImagesFixture(t, refs), resolver)
	if err == nil || !strings.Contains(err.Error(), "mixed source revisions") {
		t.Fatalf("error = %v, want mixed source revisions", err)
	}
}

func TestCheckKuraImageSuiteWrapsRegistryFailure(t *testing.T) {
	refs := testKuraRefs()
	resolver := &fakeSourceRevisionResolver{
		revisions: map[string]string{},
		errors:    map[string]error{refs["libraryManager"]: errors.New("registry unavailable")},
	}

	_, err := checkKuraImageSuite(t.Context(), writeKuraImagesFixture(t, refs), resolver)
	if err == nil || !strings.Contains(err.Error(), "libraryManager") || !strings.Contains(err.Error(), "registry unavailable") {
		t.Fatalf("error = %v, want component and registry error", err)
	}
}

func testKuraRefs() map[string]string {
	return map[string]string{
		"libraryManager": "ghcr.io/wyvernzora/kura/library-manager:main@sha256:" + strings.Repeat("a", 64),
		"gateway":        "ghcr.io/wyvernzora/kura/gateway:main@sha256:" + strings.Repeat("b", 64),
		"releaseIndexer": "ghcr.io/wyvernzora/kura/release-indexer:main@sha256:" + strings.Repeat("c", 64),
		"n8nNodes":       "ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:" + strings.Repeat("d", 64),
	}
}

func writeKuraImagesFixture(t *testing.T, refs map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "images.ts")
	var source strings.Builder
	source.WriteString("export const KURA_IMAGES = {\n")
	for _, component := range []string{"libraryManager", "gateway", "releaseIndexer", "n8nNodes"} {
		if ref, ok := refs[component]; ok {
			source.WriteString("  " + component + ": oci`" + ref + "`,\n")
		}
	}
	source.WriteString("};\n")
	if err := os.WriteFile(path, []byte(source.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
