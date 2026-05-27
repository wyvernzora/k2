package dyffignore

import (
	"strings"
	"testing"
)

func TestParseSectionedRules(t *testing.T) {
	rules, err := Parse(strings.NewReader(`
# app deploy output
[app]
/metadata/name
/spec/template/metadata/labels/helm.sh\/chart

[crd]
/metadata
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := strings.Join(rules.Section("app"), ","); got != "/metadata/name,/spec/template/metadata/labels/helm.sh\\/chart" {
		t.Fatalf("app rules: %s", got)
	}
	if got := strings.Join(rules.Section("CRD"), ","); got != "/metadata" {
		t.Fatalf("crd rules: %s", got)
	}
}

func TestParseRejectsPathBeforeSection(t *testing.T) {
	if _, err := Parse(strings.NewReader("/metadata\n")); err == nil {
		t.Fatalf("expected path-before-section error")
	}
}

func TestParseRejectsRelativePath(t *testing.T) {
	if _, err := Parse(strings.NewReader("[app]\nmetadata/name\n")); err == nil {
		t.Fatalf("expected relative path error")
	}
}

func TestParseRejectsInvalidPath(t *testing.T) {
	if _, err := Parse(strings.NewReader("[app]\n/metadata/bad=a=b\n")); err == nil {
		t.Fatalf("expected invalid path error")
	}
}

func TestParseRejectsNamedListSelectors(t *testing.T) {
	if _, err := Parse(strings.NewReader("[app]\n/spec/template/spec/containers/name=app/image\n")); err == nil {
		t.Fatalf("expected named-list selector error")
	}
}
