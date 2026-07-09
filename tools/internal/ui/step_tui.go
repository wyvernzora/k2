package ui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const scrollbackLines = 5

// spinnerFrames matches bubbles' MiniDot cadence.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	ansiCursorUpLine = "\x1b[%dF" // to column 1, n lines up
	ansiEraseBelow   = "\x1b[J"
)

// tuiStep renders an animated spinner next to the step label with a dim
// ring-buffered scrollback panel of the last N lines of subprocess output
// below, using plain in-place ANSI redraws on the shared writer.
//
// It deliberately does NOT use bubbletea: one Program per step means dozens
// of short-lived programs per run, each mutating terminal state (raw mode,
// kitty keyboard push/pop, capability queries). Interleaved with reporter
// output and process exits, that reliably wedged terminals — kitty-encoded
// Ctrl-C (^[[99;5u), orphaned query replies, swallowed lines. This renderer
// changes no terminal modes and reads no input; Ctrl-C stays a signal.
type tuiStep struct {
	r     *Reporter
	out   io.Writer
	label string

	// guarded by r.mu (all terminal writes serialize through the Reporter)
	lines     []string
	drawn     int // terminal lines currently occupied by the live block
	frame     int
	finalized bool

	writeMu sync.Mutex
	buf     bytes.Buffer

	stop chan struct{}
	done chan struct{}
}

func newTUIStep(r *Reporter, label string) *tuiStep {
	s := &tuiStep{
		r:     r,
		out:   r.out,
		label: label,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	r.mu.Lock()
	r.activeStep = s
	s.drawLocked()
	r.mu.Unlock()
	go s.tick()
	return s
}

func (s *tuiStep) tick() {
	defer close(s.done)
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.r.mu.Lock()
			if !s.finalized {
				s.frame++
				s.drawLocked()
			}
			s.r.mu.Unlock()
		}
	}
}

// eraseLocked removes the live block from the terminal. Caller holds r.mu.
func (s *tuiStep) eraseLocked() {
	if s.drawn == 0 {
		return
	}
	fmt.Fprintf(s.out, ansiCursorUpLine, s.drawn)
	fmt.Fprint(s.out, ansiEraseBelow)
	s.drawn = 0
}

// drawLocked redraws the live block in place. Caller holds r.mu.
func (s *tuiStep) drawLocked() {
	s.eraseLocked()
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(SpinnerStyle.Render(spinnerFrames[s.frame%len(spinnerFrames)]))
	b.WriteString("  ")
	b.WriteString(s.label)
	b.WriteString("\n")
	for _, line := range s.lines {
		b.WriteString("      ")
		b.WriteString(DimStyle.Render(line))
		b.WriteString("\n")
	}
	fmt.Fprint(s.out, b.String())
	s.drawn = 1 + len(s.lines)
}

func (s *tuiStep) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	s.buf.Write(p)
	var newLines []string
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
		newLines = append(newLines, line)
	}
	s.writeMu.Unlock()

	if len(newLines) > 0 {
		s.r.mu.Lock()
		if !s.finalized {
			s.lines = append(s.lines, newLines...)
			if len(s.lines) > scrollbackLines {
				s.lines = s.lines[len(s.lines)-scrollbackLines:]
			}
			s.drawLocked()
		}
		s.r.mu.Unlock()
	}
	return len(p), nil
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
	s.r.mu.Lock()
	already := s.finalized
	s.r.mu.Unlock()
	if already {
		return nil
	}
	s.Successf("%s", s.label)
	return nil
}

func (s *tuiStep) finalize(badge, format string, args ...any) {
	// Flush any partial line still buffered without a trailing newline.
	s.writeMu.Lock()
	rest := strings.TrimRight(s.buf.String(), "\r")
	s.buf.Reset()
	s.writeMu.Unlock()

	s.r.mu.Lock()
	if s.finalized {
		s.r.mu.Unlock()
		return
	}
	if rest != "" {
		s.lines = append(s.lines, rest)
		if len(s.lines) > scrollbackLines {
			s.lines = s.lines[len(s.lines)-scrollbackLines:]
		}
	}
	s.finalized = true
	if s.r.activeStep == s {
		s.r.activeStep = nil
	}
	msg := fmt.Sprintf(format, args...)
	if msg == "" {
		msg = s.label
	}
	s.eraseLocked()
	fmt.Fprintf(s.out, "  %s  %s\n", badge, msg)
	s.r.mu.Unlock()

	close(s.stop)
	<-s.done
}
