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
	// activeStep is the currently animating step, if any. While its live
	// block occupies the bottom of the terminal, stray reporter lines must
	// erase and redraw it around themselves; writing to out directly would
	// land inside the block.
	activeStep *tuiStep
}

// New constructs a Reporter writing to out, typically os.Stderr.
func New(out io.Writer, forcePlain bool) *Reporter {
	return &Reporter{
		out:   out,
		plain: forcePlain || !isTTY(out) || isCI() || noColorRequested(),
	}
}

// SetInterruptCancel registers a context-cancellation func invoked when the
// operator presses Ctrl-C inside an active terminal-controlled region. It
// returns the previously registered func: nested workflows (e2e steps that
// run a full provision workflow) must restore that on exit rather than
// setting nil, or the outer run loses Ctrl-C handling for its remainder.
func (r *Reporter) SetInterruptCancel(cancel context.CancelFunc) context.CancelFunc {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.interruptCancel
	r.interruptCancel = cancel
	return prev
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
