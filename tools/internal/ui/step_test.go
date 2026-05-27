package ui

import (
	"bytes"
	"strings"
	"testing"
)

// Plain-mode tests target the post-Direction-D output shape. Plain
// mode wraps every line with `k2-tools: <status-word> <message>` so
// CI log scrapers (and `grep`) can rely on the prefix being uniform
// across Kinds. Interactive (badge) rendering is covered by manual
// smoke tests; lipgloss color stripping in an in-memory buffer makes
// strict ANSI matching brittle.

func TestStepPlainModeWritesLabelAndStatus(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)

	step := r.Step("Running rpiboot")
	if _, err := step.Write([]byte("connecting...\n")); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if _, err := step.Write([]byte("found CM4\n")); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	step.Successf("rpiboot completed in 2.1s")

	out := buf.String()
	for _, want := range []string{
		"k2-tools: info Running rpiboot",
		"connecting...",
		"found CM4",
		"k2-tools: ok rpiboot completed in 2.1s",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("plain step output missing %q:\n%s", want, out)
		}
	}
}

func TestStepPlainModeFailDelegatesToErrorf(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	step := r.Step("Verify hash")
	step.Failf("hash mismatch")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: fail hash mismatch") {
		t.Fatalf("expected fail line, got:\n%s", out)
	}
}

func TestStepPlainModeWarnDelegatesToWarnf(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	step := r.Step("Eject")
	step.Warnf("could not eject NVMe")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: warn could not eject NVMe") {
		t.Fatalf("expected warn line, got:\n%s", out)
	}
}

func TestStepPlainModeCloseFallsBackToLabel(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	step := r.Step("Detect disks")
	if err := step.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "k2-tools: ok Detect disks") {
		t.Fatalf("expected close to fallback to label, got:\n%s", out)
	}
}

func TestStepPlainModeDoubleFinalizeIsNoop(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	step := r.Step("Twice")
	step.Successf("first")
	step.Failf("second")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: ok first") {
		t.Fatalf("expected first finalize, got:\n%s", out)
	}
	if strings.Contains(out, "second") {
		t.Fatalf("second finalize should have been ignored, got:\n%s", out)
	}
}

func TestReporterPlainModePrefix(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Infof("first line")
	r.Successf("second line")
	r.Warnf("third line")
	r.Errorf("fourth line")
	out := buf.String()
	for _, want := range []string{
		"k2-tools: info first line",
		"k2-tools: ok second line",
		"k2-tools: warn third line",
		"k2-tools: fail fourth line",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in plain output:\n%s", want, out)
		}
	}
}

func TestKeyValuefPlainMode(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.KeyValuef("Target", "%s", "ubuntu-24.04-rpi4cb")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: Target: ubuntu-24.04-rpi4cb") {
		t.Fatalf("plain key/value missing expected shape:\n%s", out)
	}
}

func TestKeyValuefInteractiveContainsBothParts(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.KeyValuef("Image", "%s", "image.raw.xz")
	out := buf.String()
	// We can't reliably assert ANSI codes (lipgloss may downgrade
	// based on terminal detection of the in-memory writer), but the
	// literal key + value text must always be present.
	if !strings.Contains(out, "Image") {
		t.Fatalf("expected styled key in output:\n%s", out)
	}
	if !strings.Contains(out, "image.raw.xz") {
		t.Fatalf("expected value in output:\n%s", out)
	}
}

func TestRenderKeyValueIsIdempotentOnTrailingColon(t *testing.T) {
	with := RenderKeyValue("foo:", "bar")
	without := RenderKeyValue("foo", "bar")
	// Both must include "foo:" exactly once — not "foo::" — once any
	// ANSI escapes are stripped. Substring match is sufficient.
	if !strings.Contains(with, "foo:") || strings.Contains(with, "foo::") {
		t.Fatalf("trailing-colon key rendered as: %q", with)
	}
	if !strings.Contains(without, "foo:") || strings.Contains(without, "foo::") {
		t.Fatalf("bare key rendered as: %q", without)
	}
}

func TestStepNonTTYFallsBackToPlain(t *testing.T) {
	// bytes.Buffer is not a *os.File, so the New() constructor's
	// `!isTTY(out)` check forces plain mode. Any Step call here must
	// therefore return a *plainStep, never a TUI step.
	var buf bytes.Buffer
	r := New(&buf, false)
	step := r.Step("Whatever")
	if _, ok := step.(*plainStep); !ok {
		t.Fatalf("expected plain step, got %T", step)
	}
	step.Successf("done")
}

func TestStepNilReporterIsSafe(t *testing.T) {
	var r *Reporter
	step := r.Step("noop")
	step.Successf("noop")
	if err := step.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestSectionPlainMode(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Section("Flash")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: == Flash ==") {
		t.Fatalf("expected plain section header, got:\n%s", out)
	}
}

func TestCheckPlainMode(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Check("disk present", CheckResult{Status: CheckOk})
	r.Check("optional dep absent", CheckResult{Status: CheckSkip, Reason: "rpiboot not installed"})
	out := buf.String()
	if !strings.Contains(out, "k2-tools: ok disk present") {
		t.Fatalf("expected ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "k2-tools: skip optional dep absent") {
		t.Fatalf("expected skip line, got:\n%s", out)
	}
}

func TestBannerPlainMode(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Banner(BannerSuccess, "Flash complete", "4.0 GB written")
	out := buf.String()
	if !strings.Contains(out, "k2-tools: banner: Flash complete") {
		t.Fatalf("expected banner line, got:\n%s", out)
	}
	if !strings.Contains(out, "k2-tools: banner: 4.0 GB written") {
		t.Fatalf("expected banner detail line, got:\n%s", out)
	}
}

func TestTablePlainMode(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Table(
		[]string{"ID", "STATE"},
		[][]string{
			{"alpha", "running"},
			{"beta", "stopped"},
		},
	)
	out := buf.String()
	if !strings.Contains(out, "k2-tools: row: alpha | running") {
		t.Fatalf("expected first row, got:\n%s", out)
	}
	if !strings.Contains(out, "k2-tools: row: beta | stopped") {
		t.Fatalf("expected second row, got:\n%s", out)
	}
}
