package ui

import (
	"context"
	"os"
)

func (w *Workflow) Section(label string) *StepHandle {
	return w.add("section "+label, func(ctx context.Context) error {
		w.reporter.Section(label)
		return nil
	})
}

func (w *Workflow) KeyValues(pairs ...KV) *StepHandle {
	return w.add("keyvalues", func(ctx context.Context) error {
		w.reporter.KeyValues(pairs...)
		return nil
	})
}

func (w *Workflow) KeyValuesFn(fn func() []KV) *StepHandle {
	return w.add("keyvalues", func(ctx context.Context) error {
		w.reporter.KeyValues(fn()...)
		return nil
	})
}

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

func (w *Workflow) Check(label string, fn func() CheckResult) *StepHandle {
	return w.add("check "+label, func(ctx context.Context) error {
		w.reporter.Check(label, fn())
		return nil
	})
}

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

func (w *Workflow) Task(label string, fn func(ctx context.Context) error) *StepHandle {
	return w.add("task "+label, func(ctx context.Context) error {
		step := w.reporter.Step(label)
		err := fn(ctx)
		if err != nil {
			step.Failf("%v", err)
		} else {
			_ = step.Close()
		}
		return err
	})
}

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

func (w *Workflow) ProgressFixed(label string, total uint64, fn func(ctx context.Context, p Progress) error) *StepHandle {
	return w.Progress(label, func() uint64 { return total }, fn)
}

func (w *Workflow) Confirm(prompt, keyword string) *StepHandle {
	return w.add("confirm "+prompt, func(ctx context.Context) error {
		return w.reporter.Confirm(os.Stdin, prompt, keyword)
	})
}

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

func (w *Workflow) Banner(kind BannerKind, lines ...string) *StepHandle {
	return w.add("banner", func(ctx context.Context) error {
		w.reporter.Banner(kind, lines...)
		return nil
	})
}

func (w *Workflow) BannerFn(kind BannerKind, fn func() []string) *StepHandle {
	return w.add("banner", func(ctx context.Context) error {
		w.reporter.Banner(kind, fn()...)
		return nil
	})
}

func (w *Workflow) Table(headers []string, rows [][]string) *StepHandle {
	return w.add("table", func(ctx context.Context) error {
		w.reporter.Table(headers, rows)
		return nil
	})
}

func (w *Workflow) TableFn(headers []string, rowsFn func() [][]string) *StepHandle {
	return w.add("table", func(ctx context.Context) error {
		w.reporter.Table(headers, rowsFn())
		return nil
	})
}

func (w *Workflow) Step(name string, fn func(ctx context.Context) error) *StepHandle {
	return w.add(name, fn)
}
