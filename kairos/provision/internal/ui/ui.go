package ui

import (
	"fmt"
	"io"
	"sync"
)

type Reporter struct {
	out   io.Writer
	plain bool
	once  sync.Once
	mu    sync.Mutex
}

func New(out io.Writer, plain bool) *Reporter {
	return &Reporter{
		out:   out,
		plain: plain,
	}
}

func (r *Reporter) Infof(format string, args ...any) {
	r.print(">", format, args...)
}

func (r *Reporter) Successf(format string, args ...any) {
	r.print("ok", format, args...)
}

func (r *Reporter) Warnf(format string, args ...any) {
	r.print("!", format, args...)
}

func (r *Reporter) Errorf(format string, args ...any) {
	r.print("x", format, args...)
}

func (r *Reporter) print(mark string, format string, args ...any) {
	if r == nil || r.out == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-provision: %s\n", message)
		return
	}
	r.once.Do(func() {
		fmt.Fprintln(r.out, "k2-provision")
	})
	fmt.Fprintf(r.out, "  %-2s %s\n", mark, message)
}
