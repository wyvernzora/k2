// Package ui owns every styled terminal output surface in k2-tools.
// All consumers reach the user through a single *Reporter, which implements
// Direction D's visual vocabulary from palette.go.
//
// Three rendering modes are honored automatically:
//   - Interactive: stderr is a TTY, --plain is unset, CI is unset, and
//     NO_COLOR is unset. Full Direction D styling with magenta accent bars,
//     status badges, MiniDot spinners, dim scrollback, and inline progress bars.
//   - Plain: any plain-mode trigger fires. Every surface degrades to flat
//     `k2-tools: ...` lines, suitable for CI logs.
//
// Adding a new step kind means adding a Reporter or Workflow method that
// respects plain mode internally. Call sites should not branch on styling.
package ui
