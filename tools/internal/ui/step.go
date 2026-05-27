package ui

import "io"

// Step is an interactive progress region. It exposes an io.Writer that
// streams subprocess output, and finalization methods that print a
// status icon + final message in place of the spinner.
//
// Steps are sequential. Do not call Step() again before the previous
// step has been finalized — k2-tools owns the terminal area while a
// step is active, and interleaving prints would corrupt the display.
type Step interface {
	io.Writer

	// Successf finalizes the step with a green check icon. The format
	// is the line shown in place of the spinner; if empty, the original
	// step label is used.
	Successf(format string, args ...any)
	// Failf finalizes the step with a red cross icon.
	Failf(format string, args ...any)
	// Warnf finalizes the step with a yellow exclamation icon.
	Warnf(format string, args ...any)
	// Close is a no-op if the step is already finalized; otherwise it
	// finalizes as Successf with the original label.
	Close() error
}

// Step returns a new Step for the duration of one subprocess (or other
// long-running operation). Choice of implementation:
//
//   - In interactive mode (stderr is a TTY, --plain is unset, and CI is
//     unset) the step animates a spinner + dim scrollback panel via the
//     bubbletea/lipgloss stack.
//   - Otherwise the step degrades to a plain reporter line at start, a
//     verbatim stdout/stderr passthrough during the run, and a final
//     status line — matching the legacy ui.Reporter shape.
func (r *Reporter) Step(label string) Step {
	if r == nil || r.out == nil {
		return &plainStep{out: io.Discard, label: label}
	}
	if r.plain {
		return newPlainStep(r, label)
	}
	return newTUIStep(r, label)
}
