package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func isCI() bool { return os.Getenv("CI") != "" }

func noColorRequested() bool { return os.Getenv("NO_COLOR") != "" }
