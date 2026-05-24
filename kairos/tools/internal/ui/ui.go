// Package ui owns every styled terminal output surface in k2-tools.
// All consumers reach the user through a single *Reporter, which
// implements Direction D's visual vocabulary (palette in palette.go).
//
// Three rendering modes are honored automatically:
//
//   - **Interactive**: stderr is a TTY, `--plain` is unset, `CI` env
//     is unset, `NO_COLOR` env is unset. Full Direction D styling
//     with magenta accent bars, status badges, MiniDot spinners,
//     dim scrollback, and inline progress bars.
//   - **Plain**: any of the above plain-mode triggers fires. Every
//     surface degrades to one flat line per event in `k2-tools: ...`
//     form, suitable for CI logs.
//
// Adding a new Step Kind = add a method here (or in a sibling file)
// that respects the plain-mode predicate the same way. Never branch
// on `if r.plain` at call sites — the call site stays uniform; the
// degradation lives in the Reporter.
package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// sudoKeepAliveInterval is well under sudo's default 5-minute
// timestamp_timeout so cached creds never lapse mid-operation. Lifted
// from internal/flash/sudo.go where this was originally tuned.
const sudoKeepAliveInterval = 60 * time.Second

// newSudoCommand is a thin wrapper around exec.CommandContext that
// keeps the sudo binary path central — useful for tests that want to
// substitute a fake.
func newSudoCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "sudo", args...)
}

// newTicker is wrapped so tests can override the cadence; today it's
// just a passthrough to time.NewTicker.
func newTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// Reporter is the central styled output sink for k2-tools.
//
// Plain mode (--plain / non-TTY / CI / NO_COLOR) emits flat lines
// prefixed with `k2-tools: `. Interactive mode emits Direction D
// styled output and supports the Step() / Progress() APIs for
// animated regions during long-running subprocesses.
type Reporter struct {
	out             io.Writer
	plain           bool
	mu              sync.Mutex
	interruptCancel context.CancelFunc
}

// New constructs a Reporter writing to `out` (typically os.Stderr).
// `forcePlain` corresponds to the `--plain` CLI flag; the Reporter
// additionally degrades to plain mode if `out` is not a TTY, if
// `CI` is set in the environment, or if `NO_COLOR` is set.
func New(out io.Writer, forcePlain bool) *Reporter {
	return &Reporter{
		out:   out,
		plain: forcePlain || !isTTY(out) || isCI() || noColorRequested(),
	}
}

// SetInterruptCancel registers a context-cancellation func that the
// Step layer invokes when the operator presses Ctrl-C inside an
// active bubbletea-controlled region. The Runner sets this once at
// startup so any `exec.CommandContext(ctx, ...)` subprocess dies
// immediately on Ctrl-C. Safe to call with nil to clear.
func (r *Reporter) SetInterruptCancel(cancel context.CancelFunc) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interruptCancel = cancel
}

// IsPlain returns true when the reporter is in flat-output mode.
// Useful for call sites that need to switch fundamental shape (not
// just styling) — e.g. emitting JSON instead of a table.
func (r *Reporter) IsPlain() bool {
	if r == nil {
		return true
	}
	return r.plain
}

func (r *Reporter) interrupt() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.interruptCancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ----- Note Kind (Infof / Successf / Warnf / Errorf) ---------------------

// Infof emits a `[ →  ]` cyan-badged narration line — the same
// channel as the older "starting X" messages but with the Direction
// D status vocabulary.
func (r *Reporter) Infof(format string, args ...any) {
	r.note(BadgeRunKind, "info", format, args...)
}

// Successf emits a `[ OK ]` green-badged success line.
func (r *Reporter) Successf(format string, args ...any) {
	r.note(BadgeOkKind, "ok", format, args...)
}

// Warnf emits a `[WARN]` yellow-badged warning line.
func (r *Reporter) Warnf(format string, args ...any) {
	r.note(BadgeWarnKind, "warn", format, args...)
}

// Errorf emits a `[FAIL]` red-badged error line. Use this for
// fatal conditions surfaced through Reporter; non-fatal anomalies
// belong on Warnf.
func (r *Reporter) Errorf(format string, args ...any) {
	r.note(BadgeFailKind, "fail", format, args...)
}

// NoteKind selects the badge style of a Note. Consumed by
// Workflow.Note; the legacy Reporter.Infof / Successf / Warnf /
// Errorf entry points are direct shortcuts for the matching NoteKind.
type NoteKind int

const (
	NoteInfo NoteKind = iota
	NoteSuccess
	NoteWarn
	NoteError
)

func (r *Reporter) note(kind BadgeKind, plainWord, format string, args ...any) {
	if r == nil || r.out == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: %s %s\n", plainWord, msg)
		return
	}
	fmt.Fprintf(r.out, "  %s  %s\n", RenderBadge(kind), msg)
}

// ----- Section Kind ------------------------------------------------------

// Section prints an inverse-video magenta header bar (`▌ LABEL`)
// followed by a dim 60-char ─ rule. Blank line above, single newline
// after. Use to separate phases of a multi-step operation.
func (r *Reporter) Section(label string) {
	if r == nil || r.out == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: == %s ==\n", label)
		return
	}
	bar := SectionBar.Render("▌ " + strings.ToUpper(label))
	rule := DimStyle.Render(strings.Repeat("─", 60))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "%s %s\n", bar, rule)
}

// ----- KeyValue Kind -----------------------------------------------------

// KV is one key/value pair for KeyValues blocks.
type KV struct {
	Key, Value string
}

// KeyValues prints aligned key/value pairs with bold-cyan keys and
// white values, two-space indent. Keys are left-aligned to the
// longest key's width.
func (r *Reporter) KeyValues(pairs ...KV) {
	if r == nil || r.out == nil || len(pairs) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		for _, p := range pairs {
			fmt.Fprintf(r.out, "k2-tools: %s: %s\n", p.Key, p.Value)
		}
		return
	}
	maxKey := 0
	for _, p := range pairs {
		if len(p.Key) > maxKey {
			maxKey = len(p.Key)
		}
	}
	for _, p := range pairs {
		key := KeyStyle.Render(fmt.Sprintf("%-*s", maxKey, p.Key))
		fmt.Fprintf(r.out, "  %s   %s\n", key, ValueStyle.Render(p.Value))
	}
}

// KeyValuef is a single-pair convenience wrapper around KeyValues
// preserved for backward compatibility. Prefer KeyValues for
// multi-pair blocks (the alignment is computed across all pairs).
func (r *Reporter) KeyValuef(key string, format string, args ...any) {
	value := fmt.Sprintf(format, args...)
	r.KeyValues(KV{Key: key, Value: value})
}

// RenderKeyValue returns the styled `<key>: <value>` string a
// single KeyValues row would emit (without the leading two-space
// indent or trailing newline). Idempotent on a trailing colon in
// `key` so callers don't have to think about it.
func RenderKeyValue(key, value string) string {
	if !strings.HasSuffix(key, ":") {
		key += ":"
	}
	return KeyStyle.Render(key) + " " + ValueStyle.Render(value)
}

// ----- Check Kind --------------------------------------------------------

// CheckStatus is the outcome of a Check probe.
type CheckStatus int

const (
	CheckOk CheckStatus = iota
	CheckSkip
	CheckWarn
	CheckFail
)

// CheckResult bundles a Check's outcome with an optional reason.
// Reason renders in dim text after a `—` separator.
type CheckResult struct {
	Status CheckStatus
	Reason string
}

// Check emits a one-line idempotency / sanity probe result. No
// spinner, no scrollback — just badge + label + optional reason.
// Use for "is X present?" / "does Y match?" style checks.
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

// ----- Banner Kind -------------------------------------------------------

// BannerKind selects the badge + text styling of a final summary.
type BannerKind int

const (
	BannerSuccess BannerKind = iota
	BannerWarn
	BannerFail
)

// Banner prints a final multi-line summary, dim `─` rule-flanked,
// first line carrying a status badge + bold magenta title text,
// subsequent lines dim and 8-space indented. Blank line above; no
// padding after. Use at the end of a multi-step operation.
func (r *Reporter) Banner(kind BannerKind, lines ...string) {
	if r == nil || r.out == nil || len(lines) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		for _, line := range lines {
			fmt.Fprintf(r.out, "k2-tools: banner: %s\n", line)
		}
		return
	}
	var badgeKind BadgeKind
	switch kind {
	case BannerWarn:
		badgeKind = BadgeWarnKind
	case BannerFail:
		badgeKind = BadgeFailKind
	default:
		badgeKind = BadgeOkKind
	}
	rule := DimStyle.Render(strings.Repeat("─", 54))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "  %s\n", rule)
	fmt.Fprintf(r.out, "  %s  %s\n", RenderBadge(badgeKind), AccentBoldStyle.Render(lines[0]))
	for _, line := range lines[1:] {
		fmt.Fprintf(r.out, "        %s\n", DimStyle.Render(line))
	}
	fmt.Fprintf(r.out, "  %s\n", rule)
}

// ----- Table Kind --------------------------------------------------------

// Table prints a header row in bold cyan, a magenta `─` rule under
// the header, and rows in default fg. Column widths are computed
// via lipgloss.Width on pre-styled cells so inline-styled prefixes
// (e.g. status dots) align correctly. Two-space indent.
//
// If a column's name contains the case-insensitive substring
// "state" or "status", the corresponding row cells get a colored
// dot prefix: green ● for "running" / "active" / "ok", dim ○ for
// anything else. This matches how Direction D's reference Table
// surfaces a VM's running/stopped distinction.
func (r *Reporter) Table(headers []string, rows [][]string) {
	if r == nil || r.out == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		r.writePlainRows(rows)
		return
	}

	stateCol := findStateColumn(headers)
	displayHeaders := renderDisplayHeaders(headers)
	displayRows := renderDisplayRows(rows, stateCol)
	widths := computeVisualWidths(displayHeaders, displayRows)

	writeTableRow(r.out, displayHeaders, widths)
	writeTableSeparator(r.out, widths)
	for _, row := range displayRows {
		writeTableRow(r.out, row, widths)
	}
}

func (r *Reporter) writePlainRows(rows [][]string) {
	for _, row := range rows {
		fmt.Fprintf(r.out, "k2-tools: row: %s\n", strings.Join(row, " | "))
	}
}

// findStateColumn returns the index of the first column whose header
// contains a case-insensitive "state" or "status" substring, or -1
// if none does. That column gets the coloured-dot prefix treatment.
func findStateColumn(headers []string) int {
	for i, h := range headers {
		low := strings.ToLower(h)
		if strings.Contains(low, "state") || strings.Contains(low, "status") {
			return i
		}
	}
	return -1
}

func renderDisplayHeaders(headers []string) []string {
	display := make([]string, len(headers))
	for i, h := range headers {
		display[i] = KeyStyle.Render(h)
	}
	return display
}

func renderDisplayRows(rows [][]string, stateCol int) [][]string {
	display := make([][]string, len(rows))
	for ri, row := range rows {
		out := make([]string, len(row))
		for ci, cell := range row {
			if ci == stateCol {
				out[ci] = renderStateCell(cell)
			} else {
				out[ci] = cell
			}
		}
		display[ri] = out
	}
	return display
}

func writeTableRow(out io.Writer, cells []string, widths []int) {
	fmt.Fprint(out, "  ")
	for i, cell := range cells {
		if i >= len(widths) {
			continue
		}
		fmt.Fprintf(out, "%s  ", padVisual(cell, widths[i]))
	}
	fmt.Fprintln(out)
}

func writeTableSeparator(out io.Writer, widths []int) {
	fmt.Fprint(out, "  ")
	for _, wd := range widths {
		fmt.Fprintf(out, "%s  ", AccentBoldStyle.Render(strings.Repeat("─", wd)))
	}
	fmt.Fprintln(out)
}

func renderStateCell(value string) string {
	low := strings.ToLower(strings.TrimSpace(value))
	if low == "running" || low == "active" || low == "ok" || low == "open" {
		return OkStyle.Render("●") + " " + value
	}
	if value == "" || low == "-" || low == "—" {
		return value
	}
	return DimStyle.Render("○") + " " + value
}

func computeVisualWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if wd := lipgloss.Width(cell); wd > widths[i] {
					widths[i] = wd
				}
			}
		}
	}
	return widths
}

// padVisual right-pads `s` with spaces until its rendered width
// (ANSI-aware) hits `width`. Spaces are unstyled so trailing
// background/foreground does not "leak" past the styled glyph.
func padVisual(s string, width int) string {
	cur := lipgloss.Width(s)
	if cur >= width {
		return s
	}
	return s + strings.Repeat(" ", width-cur)
}

// ----- Confirm Kind ------------------------------------------------------

// Confirm renders a confirmation prompt and reads a line from
// `stdin`. Returns nil iff the operator typed `requireKeyword`
// exactly (case-sensitive); returns an error otherwise. Pass
// requireKeyword = "" to accept any non-empty `y`/`yes` (case-
// insensitive) response.
//
// In plain mode the prompt + keyword are echoed and the caller
// must still provide stdin (no auto-skip — the operator can pipe
// the keyword via `echo FLASH | …`).
func (r *Reporter) Confirm(stdin io.Reader, prompt string, requireKeyword string) error {
	if r == nil || r.out == nil {
		return fmt.Errorf("nil reporter")
	}
	r.mu.Lock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: confirm: %s\n", prompt)
	} else {
		fmt.Fprintln(r.out)
		fmt.Fprintf(r.out, "  %s\n", prompt)
		fmt.Fprintf(r.out, "  %s ", AccentBoldStyle.Render(">"))
	}
	r.mu.Unlock()

	if stdin == nil {
		return fmt.Errorf("no stdin to read confirmation from")
	}
	var line string
	if _, err := fmt.Fscanln(stdin, &line); err != nil {
		// io.EOF from a closed stdin means no confirmation provided.
		return fmt.Errorf("read confirmation: %w", err)
	}
	if requireKeyword != "" {
		if strings.TrimSpace(line) != requireKeyword {
			return fmt.Errorf("aborted: expected %q, got %q", requireKeyword, line)
		}
		return nil
	}
	resp := strings.ToLower(strings.TrimSpace(line))
	if resp == "y" || resp == "yes" {
		return nil
	}
	return fmt.Errorf("aborted: confirmation declined")
}

// ----- Sudo Kind ---------------------------------------------------------

// Sudo caches the operator's sudo credentials up front so later
// subprocesses that shell out through `sudo` don't interrupt an
// active spinner or progress bar with a password prompt.
//
// The operator sees a `[ →  ]` cyan-badged "Caching sudo: <reason>"
// line, then sudo's own password prompt rendered raw (stdin/stdout/
// stderr passthrough), then a `[ OK ]` green-badged "sudo
// authenticated" line. A keep-alive goroutine then re-runs
// `sudo -n -v` every 60 s for the lifetime of `ctx` so the cached
// credentials never lapse mid-operation.
//
// Caller must defer the returned release func. The Reporter will
// refuse to start another Step or Progress while a Sudo invocation
// is mid-flight (the password prompt would race the spinner).
func (r *Reporter) Sudo(ctx context.Context, reason string) (release func(), err error) {
	if r == nil {
		return func() {}, fmt.Errorf("nil reporter")
	}
	r.Infof("Caching sudo: %s", reason)

	cmd := newSudoCommand(ctx, "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return func() {}, fmt.Errorf("sudo authentication: %w", err)
	}
	r.Successf("sudo authenticated")

	keepAliveCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := newTicker(sudoKeepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-ticker.C:
				// Best-effort refresh. The `-n` flag turns the refresh
				// non-interactive so a lapsed timestamp simply errors
				// rather than re-prompting on a terminal we don't own.
				_ = newSudoCommand(keepAliveCtx, "-n", "-v").Run()
			}
		}
	}()
	return cancel, nil
}

// ----- environment predicates -------------------------------------------

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func isCI() bool { return os.Getenv("CI") != "" }

// noColorRequested honors the cross-tool convention at
// https://no-color.org/ — any non-empty NO_COLOR env value disables
// styling, with `NO_COLOR=0` notably NOT disabling (spec is "any
// non-empty value").
func noColorRequested() bool { return os.Getenv("NO_COLOR") != "" }
