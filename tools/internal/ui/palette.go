package ui

import "charm.land/lipgloss/v2"

// Direction D palette — adopted as the canonical k2-tools style 2026-05-22.
// All styling references in this package and downstream consumers should
// reach through these exported constants/styles, never inline new
// colors at call sites. Adding a new badge kind requires extending this
// file, not papering over with a one-off lipgloss.NewStyle.
//
// Rationale notes inlined per-style so the design intent is grep-able
// alongside the code that uses it.

// ----- raw colors --------------------------------------------------------

var (
	// Magenta = primary accent. Section bar background, Banner first
	// line, progress bar fill, table header underline, confirmation
	// prompt indicator.
	Magenta = lipgloss.Color("#d75faf")

	// Cyan = secondary accent. KeyValues key text, table header text,
	// spinner foreground, "running" / "info" / "sudo narration" badge.
	Cyan = lipgloss.Color("#5fafd7")

	// Dim = soft gray for non-emphasized text: section divider rules,
	// Banner subsequent lines, Check reason text, Shell scrollback,
	// SKIP badge background.
	Dim = lipgloss.Color("8")

	// White = high-contrast text used for KeyValues values and badge
	// foregrounds.
	White = lipgloss.Color("15")

	// BarTrack = dark gray progress-bar empty track. 256-color "237";
	// reads as a faint visible strip on dark terminals so the operator
	// sees the bar's full length even at 0% fill. Operator-validated.
	BarTrack = lipgloss.Color("237")

	// Status semantic colors. Use only for status indication; never for
	// decorative purposes.
	OkColor   = lipgloss.Color("10") // green
	WarnColor = lipgloss.Color("11") // yellow
	FailColor = lipgloss.Color("9")  // red
)

// ----- shared styles -----------------------------------------------------

var (
	// SectionBar = inverse-video magenta header bar. Bold white fg on
	// magenta bg, single-space horizontal padding. Rendered as
	// "▌ FLASHING RPI4CB" then followed by a dim 60-char ─ rule.
	SectionBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(Magenta).
			Padding(0, 1)

	// DimStyle = the canonical dim text style.
	DimStyle = lipgloss.NewStyle().Foreground(Dim)

	// KeyStyle = bold-cyan style for KeyValues keys + table headers.
	KeyStyle = lipgloss.NewStyle().Bold(true).Foreground(Cyan)

	// ValueStyle = white style for KeyValues values.
	ValueStyle = lipgloss.NewStyle().Foreground(White)

	// SpinnerStyle = cyan foreground for the MiniDot spinner.
	SpinnerStyle = lipgloss.NewStyle().Foreground(Cyan)

	// AccentBoldStyle = bold magenta for Banner first lines + the
	// confirmation prompt indicator.
	AccentBoldStyle = lipgloss.NewStyle().Bold(true).Foreground(Magenta)

	// OkStyle / WarnStyle / FailStyle = unbadged status text colors,
	// used inline (e.g. coloring a "running" cell in a table).
	OkStyle   = lipgloss.NewStyle().Foreground(OkColor)
	WarnStyle = lipgloss.NewStyle().Foreground(WarnColor)
	FailStyle = lipgloss.NewStyle().Foreground(FailColor)
)

// ----- status badges -----------------------------------------------------
//
// Bold-text colored backgrounds with single-char horizontal padding.
// Used everywhere a one-shot status needs visual weight: Note Kind,
// Check Kind, Shell/Progress finalization, Sudo narration, Banner
// first line.

var (
	BadgeOk   = lipgloss.NewStyle().Bold(true).Foreground(White).Background(OkColor).Padding(0, 1)
	BadgeFail = lipgloss.NewStyle().Bold(true).Foreground(White).Background(FailColor).Padding(0, 1)
	// BadgeWarn uses BLACK foreground because white-on-yellow is
	// near-invisible on most terminal themes; black has the contrast.
	BadgeWarn = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(WarnColor).Padding(0, 1)
	BadgeRun  = lipgloss.NewStyle().Bold(true).Foreground(White).Background(Cyan).Padding(0, 1)
	BadgeSkip = lipgloss.NewStyle().Bold(true).Foreground(White).Background(Dim).Padding(0, 1)
)

// Badge labels — kept as constants so the exact spacing (e.g. trailing
// space for visual centering of `→`) is single-sourced and the badge
// surface width is uniform across kinds.
const (
	BadgeLabelOk   = " OK "
	BadgeLabelFail = "FAIL"
	BadgeLabelWarn = "WARN"
	BadgeLabelRun  = " →  "
	BadgeLabelSkip = "SKIP"
)

// RenderBadge returns the fully-styled badge string for the given
// status. Callers use this when emitting flat lines outside of an
// active TUI region; the live (Shell/Progress) Step finalizers call
// it through their own pathway.
func RenderBadge(kind BadgeKind) string {
	switch kind {
	case BadgeOkKind:
		return BadgeOk.Render(BadgeLabelOk)
	case BadgeFailKind:
		return BadgeFail.Render(BadgeLabelFail)
	case BadgeWarnKind:
		return BadgeWarn.Render(BadgeLabelWarn)
	case BadgeRunKind:
		return BadgeRun.Render(BadgeLabelRun)
	case BadgeSkipKind:
		return BadgeSkip.Render(BadgeLabelSkip)
	}
	return ""
}

// BadgeKind selects the badge style+label.
type BadgeKind int

const (
	BadgeOkKind BadgeKind = iota
	BadgeFailKind
	BadgeWarnKind
	BadgeRunKind
	BadgeSkipKind
)
