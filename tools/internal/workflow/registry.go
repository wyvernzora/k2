// Package workflow owns the operator-facing command schemas and composes
// reusable tooling operations into complete K2 workflows.
package workflow

import (
	"io"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

// Registration describes one top-level workflow command.
type Registration struct {
	Name    string
	Help    string
	Aliases []string
	Order   int
	Command any
}

// Runtime contains process-wide inputs shared by workflow executions.
type Runtime struct {
	RepoRoot string
	Jobs     int
}

var reporter = ui.New(io.Discard, true)

// NewRuntime constructs the shared runtime passed to a selected workflow.
func NewRuntime(repoRoot string, jobs int) *Runtime {
	return &Runtime{RepoRoot: repoRoot, Jobs: jobs}
}

// SetReporter configures workflow presentation for the current invocation.
func SetReporter(value *ui.Reporter) {
	reporter = value
}

// Reporter returns the presenter configured for the current invocation.
func Reporter() *ui.Reporter {
	return reporter
}
