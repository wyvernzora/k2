// Package shellscript renders the shell scripts k2-tools runs on remote nodes.
//
// These scripts run as root during provisioning and E2E runs, where a stray
// quote is expensive and hard to see. Keeping them as templates rather than
// Fprintf sequences means they read as shell, `sh -n` can parse them, and the
// quoting is visible at the point it matters.
package shellscript

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"text/template"
)

// Quote renders value as a single-quoted POSIX shell word, safe to paste into
// a command line regardless of what it contains.
func Quote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Renderer is a parsed set of script templates.
type Renderer struct {
	templates *template.Template
}

// New parses every template matching patterns out of fsys. It panics on a
// malformed template: these are embedded in the binary, so a parse failure is
// a build-time mistake rather than a runtime condition.
func New(fsys fs.FS, patterns ...string) *Renderer {
	return &Renderer{templates: template.Must(template.New("shellscript").Funcs(template.FuncMap{
		// Single-quoted for the remote shell.
		"shq": Quote,
		// Go-style double-quoted, matching the %q verb these scripts used
		// before they became templates.
		"goq": strconv.Quote,
	}).ParseFS(fsys, patterns...))}
}

// Render executes the named template. As with New, a failure here is a
// programming error in a template that ships in the binary.
func (r *Renderer) Render(name string, data any) string {
	var buf strings.Builder
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		panic(fmt.Sprintf("render shell script %s: %v", name, err))
	}
	return buf.String()
}
