package ui

import (
	"io"
	"sync"
)

// plainStep is the non-TTY / --plain / CI fallback. It mirrors the
// existing ui.Reporter line shape: print "> <label>" at start, pass
// subprocess writes through verbatim to the reporter's writer, then
// print a final status line via the matching reporter method
// (Successf / Errorf / Warnf) so plain-mode formatting stays consistent
// with everything else the binary emits.
type plainStep struct {
	reporter  *Reporter
	out       io.Writer
	label     string
	mu        sync.Mutex
	finalized bool
}

func newPlainStep(r *Reporter, label string) *plainStep {
	r.Infof("%s", label)
	return &plainStep{reporter: r, out: r.out, label: label}
}

func (s *plainStep) Write(p []byte) (int, error) {
	if s == nil || s.out == nil {
		return len(p), nil
	}
	return s.out.Write(p)
}

func (s *plainStep) Successf(format string, args ...any) {
	s.finish(s.reporter.Successf, format, args)
}

func (s *plainStep) Failf(format string, args ...any) {
	s.finish(s.reporter.Errorf, format, args)
}

func (s *plainStep) Warnf(format string, args ...any) {
	s.finish(s.reporter.Warnf, format, args)
}

func (s *plainStep) Close() error {
	s.mu.Lock()
	already := s.finalized
	s.mu.Unlock()
	if already {
		return nil
	}
	s.Successf("%s", s.label)
	return nil
}

func (s *plainStep) finish(emit func(string, ...any), format string, args []any) {
	s.mu.Lock()
	if s.finalized {
		s.mu.Unlock()
		return
	}
	s.finalized = true
	s.mu.Unlock()
	if format == "" {
		emit("%s", s.label)
		return
	}
	emit(format, args...)
}
