package ui

import (
	"context"
	"fmt"
	"os"
)

// Workflow is a declarative sequence of typed Steps that share state
// through host-language closures. Operator commands (`flash rpi4cb`,
// `provision bootstrap`, …) are built by calling builder methods —
// each appends one Step to the workflow — and then invoking
// Execute(ctx) once at the end.
//
// Why a runner instead of imperative calls into Reporter?
//   - The shape of a command becomes scannable top-down: every step
//     is one builder line. Control flow (sequencing, conditional
//     inclusion) is in the same syntactic shape as the actions.
//   - Cleanup is centralized: a Sudo step registers its release func
//     with the workflow; Execute runs deferred teardowns LIFO on
//     both success and error.
//   - Future-friendly: --dry-run can iterate w.steps and print names
//     without executing; --from-step <idx> can resume mid-flow;
//     per-step Skip predicates can express idempotent re-runs.
//
// State between steps flows through host variables captured by the
// closures, not through a Workflow-managed context. The trade-off is
// that callers see ordinary Go variable mutation; the benefit is
// zero ceremony around state typing.
type Workflow struct {
	reporter *Reporter
	steps    []*workflowStep
	deferred []func()
}

// workflowStep is a unit of work registered with the Workflow. It is
// intentionally not exported — the public surface is the builder
// methods on Workflow + the modifiers on StepHandle. Hiding the type
// keeps room to evolve the internal representation (e.g. add
// idempotency probes, per-step timing, etc.) without breaking
// callers.
type workflowStep struct {
	name string
	run  func(ctx context.Context) error
	// skip is a late-evaluated predicate. If non-nil and returns true,
	// the step is skipped at execute time without running run.
	skip func() bool
}

// StepHandle is the return value of every Workflow builder method. It
// carries a pointer into the steps slice so subsequent fluent modifiers
// (Unless / When) can attach a skip predicate to the step that was
// just appended.
type StepHandle struct {
	wf  *Workflow
	idx int
}

// NewWorkflow constructs an empty Workflow bound to a Reporter.
func NewWorkflow(r *Reporter) *Workflow {
	return &Workflow{reporter: r}
}

func (w *Workflow) add(name string, run func(ctx context.Context) error) *StepHandle {
	w.steps = append(w.steps, &workflowStep{name: name, run: run})
	return &StepHandle{wf: w, idx: len(w.steps) - 1}
}

// Defer registers a teardown function. Workflow runs registered
// teardowns LIFO when Execute returns (success or error). Use for
// resources that need cleanup outside of any specific step (Sudo
// keep-alive cancel, temp dirs, …).
func (w *Workflow) Defer(fn func()) {
	w.deferred = append(w.deferred, fn)
}

// ----- builder methods (one per Step Kind) -------------------------------

// Section registers a Section header step (the inverse-video magenta
// bar). No execution side effect beyond rendering.
func (w *Workflow) Section(label string) *StepHandle {
	return w.add("section "+label, func(ctx context.Context) error {
		w.reporter.Section(label)
		return nil
	})
}

// KeyValues registers a KeyValues block step with values fixed at
// declaration time. Use KeyValuesFn when the values depend on state
// populated by earlier steps.
func (w *Workflow) KeyValues(pairs ...KV) *StepHandle {
	return w.add("keyvalues", func(ctx context.Context) error {
		w.reporter.KeyValues(pairs...)
		return nil
	})
}

// KeyValuesFn registers a KeyValues block whose pairs are computed at
// execute time. Use when the block's contents depend on state from
// earlier steps.
func (w *Workflow) KeyValuesFn(fn func() []KV) *StepHandle {
	return w.add("keyvalues", func(ctx context.Context) error {
		w.reporter.KeyValues(fn()...)
		return nil
	})
}

// Note registers a Note Kind step (badge + one-line narration).
func (w *Workflow) Note(kind NoteKind, format string, args ...any) *StepHandle {
	return w.add("note", func(ctx context.Context) error {
		switch kind {
		case NoteSuccess:
			w.reporter.Successf(format, args...)
		case NoteWarn:
			w.reporter.Warnf(format, args...)
		case NoteError:
			w.reporter.Errorf(format, args...)
		default:
			w.reporter.Infof(format, args...)
		}
		return nil
	})
}

// Check registers a Check Kind step. The result is computed at execute
// time so checks can examine state populated by earlier steps.
func (w *Workflow) Check(label string, fn func() CheckResult) *StepHandle {
	return w.add("check "+label, func(ctx context.Context) error {
		w.reporter.Check(label, fn())
		return nil
	})
}

// Shell registers a Shell (interactive scrollback) step. The fn
// receives a Step (the Shell io.Writer + finalizers); workflow
// auto-finalizes:
//
//   - fn returns error AND didn't call sh.Failf  → Workflow calls Failf
//   - fn returns nil AND didn't call sh.Successf → Workflow calls Close
//     (which is a no-op Successf with the label)
//   - fn called Successf/Failf/Warnf itself     → Workflow's call is a no-op
//
// So callers can either return errors and let workflow finalize, or
// finalize themselves with a customized message.
func (w *Workflow) Shell(label string, fn func(ctx context.Context, sh Step) error) *StepHandle {
	return w.add("shell "+label, func(ctx context.Context) error {
		sh := w.reporter.Step(label)
		err := fn(ctx, sh)
		if err != nil {
			sh.Failf("%v", err)
		} else {
			_ = sh.Close()
		}
		return err
	})
}

// Progress registers a Progress (byte-stream live bar) step. totalFn
// is called at execute time to obtain the byte total — use this when
// the total depends on state from an earlier step. Same
// auto-finalize semantics as Shell.
func (w *Workflow) Progress(label string, totalFn func() uint64, fn func(ctx context.Context, p Progress) error) *StepHandle {
	return w.add("progress "+label, func(ctx context.Context) error {
		p := w.reporter.Progress(label, totalFn())
		err := fn(ctx, p)
		if err != nil {
			p.Failf("%v", err)
		} else {
			_ = p.Close()
		}
		return err
	})
}

// ProgressFixed is the convenience form of Progress for byte totals
// known at declaration time.
func (w *Workflow) ProgressFixed(label string, total uint64, fn func(ctx context.Context, p Progress) error) *StepHandle {
	return w.Progress(label, func() uint64 { return total }, fn)
}

// Confirm registers a Confirm step that reads from os.Stdin. When
// keyword == "", any "y"/"yes" answer is accepted; otherwise the
// operator must type the keyword exactly.
func (w *Workflow) Confirm(prompt, keyword string) *StepHandle {
	return w.add("confirm "+prompt, func(ctx context.Context) error {
		return w.reporter.Confirm(os.Stdin, prompt, keyword)
	})
}

// Sudo registers a Sudo step. The release func returned by
// Reporter.Sudo is appended to the workflow's deferred-teardown list
// so it fires LIFO when Execute returns.
func (w *Workflow) Sudo(reason string) *StepHandle {
	return w.add("sudo "+reason, func(ctx context.Context) error {
		release, err := w.reporter.Sudo(ctx, reason)
		if err != nil {
			return err
		}
		w.deferred = append(w.deferred, release)
		return nil
	})
}

// Banner registers a final-summary Banner step with lines fixed at
// declaration time.
func (w *Workflow) Banner(kind BannerKind, lines ...string) *StepHandle {
	return w.add("banner", func(ctx context.Context) error {
		w.reporter.Banner(kind, lines...)
		return nil
	})
}

// BannerFn registers a Banner whose lines are computed at execute
// time. Use when the banner content depends on state from earlier
// steps.
func (w *Workflow) BannerFn(kind BannerKind, fn func() []string) *StepHandle {
	return w.add("banner", func(ctx context.Context) error {
		w.reporter.Banner(kind, fn()...)
		return nil
	})
}

// Table registers a Table step with rows fixed at declaration time.
func (w *Workflow) Table(headers []string, rows [][]string) *StepHandle {
	return w.add("table", func(ctx context.Context) error {
		w.reporter.Table(headers, rows)
		return nil
	})
}

// TableFn registers a Table whose rows are computed at execute time.
func (w *Workflow) TableFn(headers []string, rowsFn func() [][]string) *StepHandle {
	return w.add("table", func(ctx context.Context) error {
		w.reporter.Table(headers, rowsFn())
		return nil
	})
}

// Step is the escape hatch — registers a step that doesn't fit a
// rendering Kind. `name` is for workflow listings; `fn` is the work.
// Use sparingly: prefer the typed builders (Shell, Progress, …)
// which carry rendering for free.
func (w *Workflow) Step(name string, fn func(ctx context.Context) error) *StepHandle {
	return w.add(name, fn)
}

// ----- StepHandle modifiers ---------------------------------------------

// Unless marks the step as skipped if `skip` is true. Evaluated at
// declaration time — use When for late-evaluated skip predicates.
func (h *StepHandle) Unless(skip bool) *StepHandle {
	if h == nil || h.wf == nil {
		return h
	}
	if skip {
		h.wf.steps[h.idx].skip = func() bool { return true }
	}
	return h
}

// When marks the step as skipped unless `condFn` returns true at
// execute time. Use for skip predicates that depend on state
// populated by earlier steps.
func (h *StepHandle) When(condFn func() bool) *StepHandle {
	if h == nil || h.wf == nil || condFn == nil {
		return h
	}
	h.wf.steps[h.idx].skip = func() bool { return !condFn() }
	return h
}

// ----- execute ----------------------------------------------------------

// Execute runs the workflow in declaration order. Steps with a skip
// predicate that returns true are silently skipped (no narration —
// the operator sees the omission). Execution halts on the first
// error; the error is wrapped with the step's name + index so a
// failure is locatable. Deferred teardowns run LIFO on both success
// and error paths.
func (w *Workflow) Execute(ctx context.Context) (err error) {
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

// Names returns the registered step names in declaration order. The
// runtime does not use this; it exists for `--dry-run` output and
// for tests that want to assert workflow shape without executing.
func (w *Workflow) Names() []string {
	out := make([]string, len(w.steps))
	for i, s := range w.steps {
		out[i] = s.name
	}
	return out
}
