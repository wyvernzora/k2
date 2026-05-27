package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestWorkflowExecutesStepsInOrder(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	var order []string
	wf.Step("one", func(ctx context.Context) error { order = append(order, "one"); return nil })
	wf.Step("two", func(ctx context.Context) error { order = append(order, "two"); return nil })
	wf.Step("three", func(ctx context.Context) error { order = append(order, "three"); return nil })

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := strings.Join(order, ",")
	if got != "one,two,three" {
		t.Fatalf("execution order: got %s, want one,two,three", got)
	}
}

func TestWorkflowUnlessSkipsStep(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	var ran []string
	wf.Step("kept", func(ctx context.Context) error { ran = append(ran, "kept"); return nil })
	wf.Step("skipped", func(ctx context.Context) error { ran = append(ran, "skipped"); return nil }).Unless(true)
	wf.Step("also-kept", func(ctx context.Context) error { ran = append(ran, "also-kept"); return nil })

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(ran, ",") != "kept,also-kept" {
		t.Fatalf("unexpected step execution: %v", ran)
	}
}

func TestWorkflowWhenIsLateEvaluated(t *testing.T) {
	// When(condFn) reads condFn at execute time so it can see state
	// populated by an earlier step.
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	var ready bool
	var ran string
	wf.Step("set-flag", func(ctx context.Context) error { ready = true; return nil })
	wf.Step("conditional", func(ctx context.Context) error { ran = "yes"; return nil }).
		When(func() bool { return ready })

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ran != "yes" {
		t.Fatalf("late-evaluated When did not fire after the predicate became true")
	}
}

func TestWorkflowWhenSkipsWhenFalse(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	var ran bool
	wf.Step("never", func(ctx context.Context) error { ran = true; return nil }).
		When(func() bool { return false })

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ran {
		t.Fatalf("step with When(false) ran")
	}
}

func TestWorkflowStopsAtFirstError(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	boom := errors.New("boom")
	var afterRan bool
	wf.Step("ok", func(ctx context.Context) error { return nil })
	wf.Step("fail", func(ctx context.Context) error { return boom })
	wf.Step("after", func(ctx context.Context) error { afterRan = true; return nil })

	err := wf.Execute(context.Background())
	if err == nil {
		t.Fatalf("expected error from second step")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fail") {
		t.Fatalf("expected step name in wrapped error, got: %v", err)
	}
	if afterRan {
		t.Fatalf("step after a failure should not have run")
	}
}

func TestWorkflowDeferredTeardownLIFO(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	var order []string
	wf.Defer(func() { order = append(order, "first-registered") })
	wf.Defer(func() { order = append(order, "second-registered") })
	wf.Step("work", func(ctx context.Context) error { order = append(order, "step"); return nil })

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.Join(order, ","); got != "step,second-registered,first-registered" {
		t.Fatalf("LIFO teardown order broken: %s", got)
	}
}

func TestWorkflowDeferredFiresOnError(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	boom := errors.New("boom")
	var teardownRan bool
	wf.Defer(func() { teardownRan = true })
	wf.Step("fail", func(ctx context.Context) error { return boom })

	_ = wf.Execute(context.Background())
	if !teardownRan {
		t.Fatalf("deferred teardown must run on error path too")
	}
}

func TestWorkflowContextCancellationHaltsExecution(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	var ran int
	wf.Step("one", func(ctx context.Context) error { ran++; return nil })
	wf.Step("two", func(ctx context.Context) error { ran++; return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled
	if err := wf.Execute(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if ran != 0 {
		t.Fatalf("no steps should have run on pre-cancelled context, got %d", ran)
	}
}

func TestWorkflowNamesReturnsDeclarationOrder(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	wf := NewWorkflow(r)

	wf.Section("Alpha")
	wf.Step("worker", func(ctx context.Context) error { return nil })
	wf.Banner(BannerSuccess, "done")

	names := wf.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}
	if !strings.Contains(names[0], "Alpha") || !strings.Contains(names[1], "worker") || !strings.Contains(names[2], "banner") {
		t.Fatalf("names not in declaration order: %v", names)
	}
}

func TestWorkflowKindBuildersRenderInPlainMode(t *testing.T) {
	// Spot-check that builder methods route to the right Reporter
	// surface in plain mode. Full rendering is covered by Reporter's
	// own tests; this confirms the workflow doesn't drop kinds.
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	wf.Section("Phase")
	wf.KeyValues(KV{Key: "k", Value: "v"})
	wf.Note(NoteInfo, "hello %s", "world")
	wf.Check("disk present", func() CheckResult { return CheckResult{Status: CheckOk} })
	wf.Banner(BannerSuccess, "done")

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"k2-tools: == Phase ==",
		"k2-tools: k: v",
		"k2-tools: info hello world",
		"k2-tools: ok disk present",
		"k2-tools: banner: done",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestWorkflowShellAutoFinalizesOnError(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	boom := errors.New("oops")
	wf.Shell("Try thing", func(ctx context.Context, sh Step) error {
		// Don't call sh.Failf — workflow should do it.
		return boom
	})

	err := wf.Execute(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "fail oops") {
		t.Fatalf("expected workflow to auto-Failf on returned error:\n%s", out)
	}
}

func TestWorkflowShellAutoFinalizesOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	wf.Shell("Quick check", func(ctx context.Context, sh Step) error {
		// Don't finalize; workflow should Close (== Successf with label).
		return nil
	})

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok Quick check") {
		t.Fatalf("expected workflow to auto-Successf with label:\n%s", out)
	}
}

func TestWorkflowShellRespectsExplicitSuccessfMessage(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	wf.Shell("Probe", func(ctx context.Context, sh Step) error {
		sh.Successf("found 3 disks")
		return nil
	})

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok found 3 disks") {
		t.Fatalf("expected custom Successf message, got:\n%s", out)
	}
	if strings.Contains(out, "ok Probe") {
		t.Fatalf("workflow should not have appended its own Successf after explicit one:\n%s", out)
	}
}

func TestWorkflowTaskRendersTypedStep(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	var ran bool
	wf.Task("Compute plan", func(ctx context.Context) error {
		ran = true
		return nil
	})

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !ran {
		t.Fatalf("task did not run")
	}
	out := buf.String()
	if !strings.Contains(out, "info Compute plan") || !strings.Contains(out, "ok Compute plan") {
		t.Fatalf("task did not render start and success:\n%s", out)
	}
}

func TestWorkflowJobGroupAggregatesFailures(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	boom := errors.New("boom")
	wf.JobGroup("Parallel checks", func() []JobSpec {
		return []JobSpec{
			{
				Name: "ok",
				Run: func(ctx context.Context, out io.Writer) error {
					_, _ = out.Write([]byte("all good\n"))
					return nil
				},
			},
			{
				Name: "bad",
				Run: func(ctx context.Context, out io.Writer) error {
					_, _ = out.Write([]byte("useful tail\n"))
					return boom
				},
			},
		}
	}, JobGroupOptions{Concurrency: 2})

	err := wf.Execute(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected aggregated boom, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad: boom") || !strings.Contains(err.Error(), "useful tail") {
		t.Fatalf("missing job name/output tail in error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"k2-tools: info Parallel checks: running 2 jobs",
		"k2-tools: row: ok | ok |",
		"k2-tools: row: bad | failed |",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestWorkflowJobGroupSurfacesSuccessfulWarnings(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	wf.JobGroup("Parallel checks", func() []JobSpec {
		return []JobSpec{
			{
				Name: "ok",
				Run: func(ctx context.Context, out io.Writer) error {
					_, _ = out.Write([]byte("warning: pay attention\nplain detail\n"))
					return nil
				},
			},
		}
	}, JobGroupOptions{Concurrency: 1})

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"k2-tools: row: ok | ok |",
		"k2-tools: warn ok: pay attention",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "plain detail") {
		t.Fatalf("non-warning successful job output should stay captured, not echoed:\n%s", out)
	}
}

func TestWorkflowJobGroupSurfacesSuccessfulWarningsBeyondTail(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	wf.JobGroup("Parallel checks", func() []JobSpec {
		return []JobSpec{
			{
				Name: "ok",
				Run: func(ctx context.Context, out io.Writer) error {
					_, _ = out.Write([]byte("warning: early warning\n"))
					for i := range 12 {
						_, _ = fmt.Fprintf(out, "detail %d\n", i)
					}
					return nil
				},
			},
		}
	}, JobGroupOptions{Concurrency: 1, OutputTailLines: 2})

	if err := wf.Execute(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "k2-tools: warn ok: early warning") {
		t.Fatalf("early warning beyond tail was not surfaced:\n%s", out)
	}
	if strings.Contains(out, "detail 11") {
		t.Fatalf("non-warning successful tail output should not be echoed:\n%s", out)
	}
}

func TestWorkflowJobGroupReportsUnstartedJobsAsCanceled(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	wf := NewWorkflow(r)

	boom := errors.New("boom")
	wf.JobGroup("Parallel checks", func() []JobSpec {
		return []JobSpec{
			{
				Name: "bad",
				Run: func(ctx context.Context, out io.Writer) error {
					return boom
				},
			},
			{
				Name: "unstarted",
				Run: func(ctx context.Context, out io.Writer) error {
					t.Fatalf("second job should not start after fail-fast cancellation")
					return nil
				},
			},
		}
	}, JobGroupOptions{Concurrency: 1, FailFast: true})

	err := wf.Execute(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected original failure, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "k2-tools: row: bad | failed |") {
		t.Fatalf("missing failed row:\n%s", out)
	}
	if !strings.Contains(out, "k2-tools: row: unstarted | canceled |") {
		t.Fatalf("unstarted job should be reported as canceled:\n%s", out)
	}
}
