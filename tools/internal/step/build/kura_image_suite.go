package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	clientoci "github.com/wyvernzora/k2/tools/internal/client/oci"
)

type sourceRevisionResolver interface {
	SourceRevision(context.Context, string) (string, error)
}

type kuraImageComponent struct {
	name       string
	repository string
}

var kuraImageComponents = []kuraImageComponent{
	{name: "libraryManager", repository: "ghcr.io/wyvernzora/kura/library-manager"},
	{name: "gateway", repository: "ghcr.io/wyvernzora/kura/gateway"},
	{name: "releaseIndexer", repository: "ghcr.io/wyvernzora/kura/release-indexer"},
	{name: "n8nNodes", repository: "ghcr.io/wyvernzora/kura/n8n-nodes"},
}

var (
	kuraImageAssignmentPattern = regexp.MustCompile("(?m)^\\s*([A-Za-z][A-Za-z0-9]*):\\s*oci`([^`]+)`,\\s*$")
	gitRevisionPattern         = regexp.MustCompile("^[0-9a-f]{40}$")
)

// CheckKuraImageSuite verifies that all Kura image pins resolve to one source revision.
func CheckKuraImageSuite(ctx context.Context, repoRoot string) (string, error) {
	return checkKuraImageSuite(
		ctx,
		filepath.Join(repoRoot, "apps", "kura", "images.ts"),
		clientoci.New(),
	)
}

func checkKuraImageSuite(ctx context.Context, sourcePath string, resolver sourceRevisionResolver) (string, error) {
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("read Kura image suite: %w", err)
	}
	refs, err := parseKuraImageRefs(string(source))
	if err != nil {
		return "", err
	}

	revisions := make(map[string]string, len(kuraImageComponents))
	for _, component := range kuraImageComponents {
		ref := refs[component.name]
		revision, err := resolver.SourceRevision(ctx, ref)
		if err != nil {
			return "", fmt.Errorf("inspect Kura %s image: %w", component.name, err)
		}
		if !gitRevisionPattern.MatchString(revision) {
			return "", fmt.Errorf("Kura %s image has invalid source revision %q", component.name, revision)
		}
		revisions[component.name] = revision
	}

	want := revisions[kuraImageComponents[0].name]
	for _, component := range kuraImageComponents[1:] {
		if revisions[component.name] != want {
			return "", fmt.Errorf("Kura image suite has mixed source revisions: %s", formatKuraRevisions(revisions))
		}
	}
	return want, nil
}

func parseKuraImageRefs(source string) (map[string]string, error) {
	refs := make(map[string]string, len(kuraImageComponents))
	for _, match := range kuraImageAssignmentPattern.FindAllStringSubmatch(source, -1) {
		name, ref := match[1], match[2]
		if _, exists := refs[name]; exists {
			return nil, fmt.Errorf("Kura image suite contains duplicate %s pin", name)
		}
		refs[name] = ref
	}
	if len(refs) != len(kuraImageComponents) {
		for _, component := range kuraImageComponents {
			if refs[component.name] == "" {
				return nil, fmt.Errorf("Kura image suite is missing %s pin", component.name)
			}
		}
		return nil, fmt.Errorf("Kura image suite contains unexpected pins")
	}
	for _, component := range kuraImageComponents {
		ref := refs[component.name]
		pattern := regexp.MustCompile("^" + regexp.QuoteMeta(component.repository) + ":main@sha256:[0-9a-f]{64}$")
		if !pattern.MatchString(ref) {
			return nil, fmt.Errorf("Kura %s image must be pinned as %s:main@sha256:<digest>, got %q", component.name, component.repository, ref)
		}
	}
	return refs, nil
}

func formatKuraRevisions(revisions map[string]string) string {
	parts := make([]string, 0, len(kuraImageComponents))
	for _, component := range kuraImageComponents {
		parts = append(parts, component.name+"="+revisions[component.name])
	}
	return strings.Join(parts, ", ")
}
