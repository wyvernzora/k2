package ui

import (
	"context"
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
