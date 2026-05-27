package ui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const scrollbackLines = 5

// tuiStep drives a bubbletea program that renders an animated MiniDot
// spinner next to the step label, with a dim ring-buffered scrollback
// panel of the last N lines of subprocess output below. Finalization
// (Successf/Failf/Warnf) replaces the spinner area with a static
// `  <badge>  <message>` line that persists in scrollback.
type tuiStep struct {
	prog  *tea.Program
	done  chan error
	out   io.Writer
	label string

	writeMu sync.Mutex
	buf     bytes.Buffer

	finalMu   sync.Mutex
	finalized bool
}

func newTUIStep(r *Reporter, label string) *tuiStep {
	model := newStepModel(label, r)
	prog := tea.NewProgram(
		model,
		tea.WithOutput(r.out),
	)
	step := &tuiStep{
		prog:  prog,
		done:  make(chan error, 1),
		out:   r.out,
		label: label,
	}
	go func() {
		_, err := prog.Run()
		step.done <- err
		close(step.done)
	}()
	return step
}

func (s *tuiStep) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	s.buf.Write(p)
	for {
		raw := s.buf.Bytes()
		if len(raw) == 0 {
			break
		}
		// Split on EITHER \n or \r. `dd status=progress` rewrites the
		// same line via \r and never emits \n until the run finishes;
		// without \r-splitting the scrollback shows nothing during dd.
		idx := bytes.IndexAny(raw, "\r\n")
		if idx < 0 {
			break
		}
		line := string(raw[:idx])
		consumed := idx + 1
		if raw[idx] == '\r' && idx+1 < len(raw) && raw[idx+1] == '\n' {
			consumed = idx + 2
		}
		s.buf.Next(consumed)
		if line == "" {
			continue
		}
		s.writeMu.Unlock()
		s.sendLine(line)
		s.writeMu.Lock()
	}
	s.writeMu.Unlock()
	return len(p), nil
}

func (s *tuiStep) sendLine(line string) {
	s.prog.Send(stepLineMsg(line))
}

func (s *tuiStep) Successf(format string, args ...any) {
	s.finalize(RenderBadge(BadgeOkKind), format, args...)
}

func (s *tuiStep) Failf(format string, args ...any) {
	s.finalize(RenderBadge(BadgeFailKind), format, args...)
}

func (s *tuiStep) Warnf(format string, args ...any) {
	s.finalize(RenderBadge(BadgeWarnKind), format, args...)
}

func (s *tuiStep) Close() error {
	s.finalMu.Lock()
	already := s.finalized
	s.finalMu.Unlock()
	if already {
		return nil
	}
	s.Successf("%s", s.label)
	return nil
}

func (s *tuiStep) finalize(badge, format string, args ...any) {
	s.finalMu.Lock()
	if s.finalized {
		s.finalMu.Unlock()
		return
	}
	s.finalized = true
	s.finalMu.Unlock()

	// Flush any partial line still buffered without a trailing newline.
	s.writeMu.Lock()
	if s.buf.Len() > 0 {
		rest := strings.TrimRight(s.buf.String(), "\r")
		s.buf.Reset()
		s.writeMu.Unlock()
		s.sendLine(rest)
	} else {
		s.writeMu.Unlock()
	}

	msg := fmt.Sprintf(format, args...)
	if msg == "" {
		msg = s.label
	}
	s.prog.Send(stepFinalMsg{})
	s.prog.Quit()
	<-s.done
	// After the bubbletea program tears down, print the final status
	// line as a flat record so it survives in the scrollback.
	fmt.Fprintf(s.out, "  %s  %s\n", badge, msg)
}

// stepModel is the bubbletea Model used by tuiStep. Spinner = MiniDot
// (10-frame braille `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, ora-default), styled cyan.
type stepModel struct {
	spinner  spinner.Model
	label    string
	lines    []string
	maxLines int
	reporter *Reporter // for surfacing Ctrl-C up to the runner's cancel func
}

type stepLineMsg string
type stepFinalMsg struct{}

func newStepModel(label string, reporter *Reporter) stepModel {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = SpinnerStyle
	return stepModel{
		spinner:  sp,
		label:    label,
		lines:    make([]string, 0, scrollbackLines),
		maxLines: scrollbackLines,
		reporter: reporter,
	}
}

func (m stepModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m stepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Bubbletea puts the terminal in raw mode, so Ctrl-C arrives as
		// a key event rather than SIGINT. Invoke the reporter's
		// interrupt-cancel func — this kills any subprocess started
		// with `exec.CommandContext` against the same context.
		if msg.Type == tea.KeyCtrlC {
			m.reporter.interrupt()
			return m, tea.Quit
		}
		return m, nil
	case stepLineMsg:
		m.lines = append(m.lines, string(msg))
		if len(m.lines) > m.maxLines {
			m.lines = m.lines[len(m.lines)-m.maxLines:]
		}
		return m, nil
	case stepFinalMsg:
		// Render an empty final frame; the driver prints the real
		// status line directly to the writer after Quit().
		m.lines = nil
		m.label = ""
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m stepModel) View() string {
	if m.label == "" && len(m.lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(m.spinner.View())
	b.WriteString("  ")
	b.WriteString(m.label)
	b.WriteString("\n")
	for _, line := range m.lines {
		b.WriteString("      ")
		b.WriteString(DimStyle.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}
