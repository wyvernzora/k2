package e2e

import (
	"strings"
	"testing"
)

func TestE2EIdentityCaptureScriptCoversEveryFact(t *testing.T) {
	got := e2eCaptureIdentityScript()
	for _, fact := range e2eIdentityFacts {
		if !strings.Contains(got, "printf '"+fact.name+"=%s\\n'") {
			t.Fatalf("capture script does not emit fact %q:\n%s", fact.name, got)
		}
	}
}

func TestE2EIdentityFactsRoundTrip(t *testing.T) {
	facts := parseE2EIdentityFacts("iqn=iqn.1993-08.org.debian:01:abc\nmachine-id=deadbeef\n\nnoise\n")
	if facts["iqn"] != "iqn.1993-08.org.debian:01:abc" {
		t.Fatalf("iqn = %q", facts["iqn"])
	}
	if facts["machine-id"] != "deadbeef" {
		t.Fatalf("machine-id = %q", facts["machine-id"])
	}
	if len(facts) != 2 {
		t.Fatalf("parsed %d facts, want 2: %#v", len(facts), facts)
	}
}

// The regression this whole scenario exists for: an upgrade replacing the
// node's IQN with the one baked into the image.
func TestE2EIdentityDiffCatchesClobberedIQN(t *testing.T) {
	before := map[string]string{"iqn": "iqn.1993-08.org.debian:01:91d199083b", "machine-id": "same"}
	after := map[string]string{"iqn": "iqn.2004-10.com.ubuntu:01:3a4942d2333d", "machine-id": "same"}

	changed := diffE2EIdentityFacts(before, after)
	if len(changed) != 1 || !strings.HasPrefix(changed[0], "iqn: ") {
		t.Fatalf("diff = %#v, want a single iqn change", changed)
	}
	if len(diffE2EIdentityFacts(before, before)) != 0 {
		t.Fatal("identical fact sets must not report a change")
	}
}

// An unreadable path yields "" on both sides and compares equal, so a broken
// capture would otherwise look like a perfectly preserved identity.
func TestE2EIdentityRequiresNonEmptyFactsBeforeUpgrade(t *testing.T) {
	missing := missingE2EIdentityFacts(map[string]string{"iqn": "", "machine-id": "deadbeef"})
	if len(missing) != 1 || missing[0] != "iqn" {
		t.Fatalf("missing = %#v, want [iqn]", missing)
	}
	if got := missingE2EIdentityFacts(map[string]string{"iqn": "x", "machine-id": "y"}); len(got) != 0 {
		t.Fatalf("missing = %#v, want none", got)
	}
}
