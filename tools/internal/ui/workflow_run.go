package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func (h *StepHandle) Unless(skip bool) *StepHandle {
	if h == nil || h.wf == nil {
		return h
	}
	if skip {
		h.wf.steps[h.idx].skip = func() bool { return true }
	}
	return h
}

func (h *StepHandle) When(condFn func() bool) *StepHandle {
	if h == nil || h.wf == nil || condFn == nil {
		return h
	}
	h.wf.steps[h.idx].skip = func() bool { return !condFn() }
	return h
}

func (w *Workflow) Execute(ctx context.Context) (err error) {
	// Steps no longer put the terminal in raw mode, so Ctrl-C arrives as a
	// real SIGINT: route the first one through the reporter's interrupt
	// cancel (graceful context cancellation); a second Ctrl-C falls through
	// to the default handler and kills the process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	sigDone := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			w.reporter.interrupt()
			signal.Stop(sigCh)
		case <-sigDone:
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(sigDone)
	}()
	defer func() {
		for i := len(w.deferred) - 1; i >= 0; i-- {
			w.deferred[i]()
		}
	}()

	for i, s := range w.steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.skip != nil && s.skip() {
			continue
		}
		if e := s.run(ctx); e != nil {
			return fmt.Errorf("workflow step %d (%s): %w", i, s.name, e)
		}
	}
	return nil
}

func (w *Workflow) Names() []string {
	out := make([]string, len(w.steps))
	for i, s := range w.steps {
		out[i] = s.name
	}
	return out
}
