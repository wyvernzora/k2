package ui

import "fmt"

// Infof emits a cyan-badged narration line.
func (r *Reporter) Infof(format string, args ...any) {
	r.note(BadgeRunKind, "info", format, args...)
}

// Successf emits a green-badged success line.
func (r *Reporter) Successf(format string, args ...any) {
	r.note(BadgeOkKind, "ok", format, args...)
}

// Warnf emits a yellow-badged warning line.
func (r *Reporter) Warnf(format string, args ...any) {
	r.note(BadgeWarnKind, "warn", format, args...)
}

// Errorf emits a red-badged error line.
func (r *Reporter) Errorf(format string, args ...any) {
	r.note(BadgeFailKind, "fail", format, args...)
}

// NoteKind selects the badge style of a Note.
type NoteKind int

const (
	NoteInfo NoteKind = iota
	NoteSuccess
	NoteWarn
	NoteError
)

func (r *Reporter) note(kind BadgeKind, plainWord, format string, args ...any) {
	if r == nil || r.out == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: %s %s\n", plainWord, msg)
		return
	}
	fmt.Fprintf(r.out, "  %s  %s\n", RenderBadge(kind), msg)
}
