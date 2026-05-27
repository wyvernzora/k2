package ui

import "fmt"

// CheckStatus is the outcome of a Check probe.
type CheckStatus int

const (
	CheckOk CheckStatus = iota
	CheckSkip
	CheckWarn
	CheckFail
)

// CheckResult bundles a Check's outcome with an optional reason.
type CheckResult struct {
	Status CheckStatus
	Reason string
}

// Check emits a one-line idempotency or sanity probe result.
func (r *Reporter) Check(label string, result CheckResult) {
	if r == nil || r.out == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: %s %s\n", checkPlainWord(result.Status), label)
		return
	}
	badge := RenderBadge(checkBadgeKind(result.Status))
	suffix := ""
	if result.Reason != "" {
		suffix = DimStyle.Render("  — " + result.Reason)
	}
	fmt.Fprintf(r.out, "  %s  %s%s\n", badge, label, suffix)
}

func checkBadgeKind(s CheckStatus) BadgeKind {
	switch s {
	case CheckOk:
		return BadgeOkKind
	case CheckSkip:
		return BadgeSkipKind
	case CheckWarn:
		return BadgeWarnKind
	case CheckFail:
		return BadgeFailKind
	}
	return BadgeRunKind
}

func checkPlainWord(s CheckStatus) string {
	switch s {
	case CheckOk:
		return "ok"
	case CheckSkip:
		return "skip"
	case CheckWarn:
		return "warn"
	case CheckFail:
		return "fail"
	}
	return "?"
}
