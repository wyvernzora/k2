package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Progress is the Step Kind for byte-stream operations with a known
// total. It is an io.Writer onto which the caller streams the bytes
// it processes; the Reporter renders a magenta bar + bytes/rate/ETA
// line below the spinner.
//
// Use this for any "writing N bytes" / "verifying N bytes" / "copying
// N bytes" surface. For interactive narrative output without a known
// total, use Step instead.
type Progress interface {
	io.Writer

	// Successf finalizes the progress step with a green [ OK ] badge.
	Successf(format string, args ...any)
	// Failf finalizes the step with a red [FAIL] badge.
	Failf(format string, args ...any)
	// Warnf finalizes the step with a yellow [WARN] badge.
	Warnf(format string, args ...any)
	// Close finalizes as Successf with the original label if not yet
	// finalized.
	Close() error
}

// Progress starts a new live progress step. `total` is the expected
// byte count over the lifetime of this step; the bar visualizes the
// written count as a fraction of total, and the ETA derives from
// (total - written) / current rate.
//
// Plain-mode (and CI/NO_COLOR) Progress emits one start line, a
// final result line, but no animation.
func (r *Reporter) Progress(label string, total uint64) Progress {
	if r == nil || r.out == nil {
		return &nopProgress{}
	}
	if r.plain {
		return newPlainProgress(r, label, total)
	}
	return newTUIProgress(r, label, total)
}

// ----- TUI progress ------------------------------------------------------

type tuiProgress struct {
	prog      *tea.Program
	done      chan error
	out       io.Writer
	label     string
	total     uint64
	count     atomic.Uint64
	startTime time.Time
	finalMu   sync.Mutex
	finalized bool
}

func newTUIProgress(r *Reporter, label string, total uint64) *tuiProgress {
	tp := &tuiProgress{
		done:      make(chan error, 1),
		out:       r.out,
		label:     label,
		total:     total,
		startTime: time.Now(),
	}
	model := newProgressModel(label, total, tp, r)
	prog := tea.NewProgram(model, tea.WithOutput(r.out))
	tp.prog = prog
	go func() {
		_, err := prog.Run()
		tp.done <- err
		close(tp.done)
	}()
	return tp
}

func (p *tuiProgress) Write(b []byte) (int, error) {
	p.count.Add(uint64(len(b)))
	// Don't send a message per write — the periodic Tick re-renders
	// at a stable cadence; flooding the program with messages on a
	// fast stream just thrashes the renderer.
	return len(b), nil
}

func (p *tuiProgress) Successf(format string, args ...any) {
	p.finalize(RenderBadge(BadgeOkKind), format, args...)
}

func (p *tuiProgress) Failf(format string, args ...any) {
	p.finalize(RenderBadge(BadgeFailKind), format, args...)
}

func (p *tuiProgress) Warnf(format string, args ...any) {
	p.finalize(RenderBadge(BadgeWarnKind), format, args...)
}

func (p *tuiProgress) Close() error {
	p.finalMu.Lock()
	already := p.finalized
	p.finalMu.Unlock()
	if already {
		return nil
	}
	p.Successf("%s", p.label)
	return nil
}

func (p *tuiProgress) finalize(badge, format string, args ...any) {
	p.finalMu.Lock()
	if p.finalized {
		p.finalMu.Unlock()
		return
	}
	p.finalized = true
	p.finalMu.Unlock()

	msg := fmt.Sprintf(format, args...)
	if msg == "" {
		msg = p.label
	}
	p.prog.Send(progressFinalMsg{})
	p.prog.Quit()
	<-p.done
	fmt.Fprintf(p.out, "  %s  %s\n", badge, msg)
}

// ----- bubbletea model for progress --------------------------------------

type progressTickMsg time.Time
type progressFinalMsg struct{}

type progressModel struct {
	spinner  spinner.Model
	label    string
	total    uint64
	parent   *tuiProgress
	reporter *Reporter
	done     bool
}

func newProgressModel(label string, total uint64, parent *tuiProgress, reporter *Reporter) progressModel {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = SpinnerStyle
	return progressModel{
		spinner:  sp,
		label:    label,
		total:    total,
		parent:   parent,
		reporter: reporter,
	}
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return progressTickMsg(t) }),
	)
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.reporter.interrupt()
			return m, tea.Quit
		}
		return m, nil
	case progressTickMsg:
		if m.done {
			return m, nil
		}
		return m, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return progressTickMsg(t) })
	case progressFinalMsg:
		m.done = true
		m.label = ""
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m progressModel) View() tea.View {
	if m.label == "" {
		return tea.NewView("")
	}
	written := m.parent.count.Load()
	var pct float64
	if m.total > 0 {
		pct = float64(written) / float64(m.total)
	}
	if pct > 1 {
		pct = 1
	}
	elapsed := time.Since(m.parent.startTime)
	var rate float64
	if elapsed > 50*time.Millisecond {
		rate = float64(written) / elapsed.Seconds()
	}
	eta := ""
	if rate > 0 && written < m.total {
		rem := time.Duration(float64(m.total-written)/rate) * time.Second
		eta = " · ETA " + HumanDuration(rem)
	}
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(m.spinner.View())
	b.WriteString("  ")
	b.WriteString(m.label)
	b.WriteString("\n      ")
	b.WriteString(renderProgressBar(pct, 30))
	b.WriteString("  ")
	b.WriteString(DimStyle.Render(fmt.Sprintf(
		"%s / %s  %5.1f%%  %s%s",
		HumanBytes(written),
		HumanBytes(m.total),
		pct*100,
		HumanRate(rate),
		eta,
	)))
	b.WriteString("\n")
	return tea.NewView(b.String())
}

// renderProgressBar draws Direction D's 30-cell magenta-on-dark-gray
// bar with sub-cell granularity. Empty cells are bg-colored spaces
// (not `░` glyphs) so the full bar length is always visible as a
// faint track.
func renderProgressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	cells := pct * float64(width)
	full := int(cells)
	frac := cells - float64(full)
	subs := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	subIdx := int(frac * float64(len(subs)))
	if subIdx >= len(subs) {
		subIdx = len(subs) - 1
	}
	track := lipgloss.NewStyle().Background(BarTrack)
	fillStyle := lipgloss.NewStyle().Foreground(Magenta).Background(BarTrack)
	filled := fillStyle.Render(strings.Repeat("█", full))
	if full >= width {
		return filled
	}
	partial := fillStyle.Render(subs[subIdx])
	empty := track.Render(strings.Repeat(" ", width-full-1))
	return filled + partial + empty
}

// ----- plain-mode progress -----------------------------------------------

type plainProgress struct {
	reporter  *Reporter
	label     string
	total     uint64
	count     atomic.Uint64
	startTime time.Time
	finalMu   sync.Mutex
	finalized bool
}

func newPlainProgress(r *Reporter, label string, total uint64) *plainProgress {
	r.Infof("%s (total %s)", label, HumanBytes(total))
	return &plainProgress{reporter: r, label: label, total: total, startTime: time.Now()}
}

func (p *plainProgress) Write(b []byte) (int, error) {
	p.count.Add(uint64(len(b)))
	return len(b), nil
}

func (p *plainProgress) Successf(format string, args ...any) {
	p.finalize(p.reporter.Successf, format, args...)
}

func (p *plainProgress) Failf(format string, args ...any) {
	p.finalize(p.reporter.Errorf, format, args...)
}

func (p *plainProgress) Warnf(format string, args ...any) {
	p.finalize(p.reporter.Warnf, format, args...)
}

func (p *plainProgress) Close() error {
	p.finalMu.Lock()
	already := p.finalized
	p.finalMu.Unlock()
	if already {
		return nil
	}
	p.Successf("%s", p.label)
	return nil
}

func (p *plainProgress) finalize(emit func(string, ...any), format string, args ...any) {
	p.finalMu.Lock()
	if p.finalized {
		p.finalMu.Unlock()
		return
	}
	p.finalized = true
	p.finalMu.Unlock()
	written := p.count.Load()
	if format == "" {
		emit("%s (%s)", p.label, HumanBytes(written))
		return
	}
	emit("%s (%s)", fmt.Sprintf(format, args...), HumanBytes(written))
}

// nopProgress is returned when the reporter is nil or has no writer;
// callers can treat the Progress as a sink without nil checks.
type nopProgress struct{}

func (n *nopProgress) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopProgress) Successf(string, ...any)     {}
func (n *nopProgress) Failf(string, ...any)        {}
func (n *nopProgress) Warnf(string, ...any)        {}
func (n *nopProgress) Close() error                { return nil }

// ----- byte formatters (shared with Progress; future home of any other
// io-formatter helpers in this package) -----------------------------------

func HumanBytes(n uint64) string {
	const (
		KB = 1000.0
		MB = 1000 * KB
		GB = 1000 * MB
	)
	f := float64(n)
	switch {
	case f >= GB:
		return fmt.Sprintf("%.1f GB", f/GB)
	case f >= MB:
		return fmt.Sprintf("%.1f MB", f/MB)
	case f >= KB:
		return fmt.Sprintf("%.1f kB", f/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func HumanRate(bps float64) string {
	const (
		KB = 1000.0
		MB = 1000 * KB
		GB = 1000 * MB
	)
	switch {
	case bps >= GB:
		return fmt.Sprintf("%.1f GB/s", bps/GB)
	case bps >= MB:
		return fmt.Sprintf("%.1f MB/s", bps/MB)
	case bps >= KB:
		return fmt.Sprintf("%.1f kB/s", bps/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

func HumanDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	if d < time.Hour {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Minute).String()
}
