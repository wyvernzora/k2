package ui

import (
	"context"
	"io"
	"sync"
)

// Reporter is the central styled output sink for k2-tools.
type Reporter struct {
	out             io.Writer
	plain           bool
	mu              sync.Mutex
	interruptCancel context.CancelFunc
}

// New constructs a Reporter writing to out, typically os.Stderr.
func New(out io.Writer, forcePlain bool) *Reporter {
	return &Reporter{
		out:   out,
		plain: forcePlain || !isTTY(out) || isCI() || noColorRequested(),
	}
}

// SetInterruptCancel registers a context-cancellation func invoked when the
// operator presses Ctrl-C inside an active terminal-controlled region.
func (r *Reporter) SetInterruptCancel(cancel context.CancelFunc) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interruptCancel = cancel
}

// IsPlain returns true when the reporter is in flat-output mode.
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
