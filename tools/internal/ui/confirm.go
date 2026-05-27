package ui

import (
	"fmt"
	"io"
	"strings"
)

// Confirm renders a confirmation prompt and reads one response line.
func (r *Reporter) Confirm(stdin io.Reader, prompt string, requireKeyword string) error {
	if r == nil || r.out == nil {
		return fmt.Errorf("nil reporter")
	}
	r.mu.Lock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: confirm: %s\n", prompt)
	} else {
		fmt.Fprintln(r.out)
		fmt.Fprintf(r.out, "  %s\n", prompt)
		fmt.Fprintf(r.out, "  %s ", AccentBoldStyle.Render(">"))
	}
	r.mu.Unlock()

	if stdin == nil {
		return fmt.Errorf("no stdin to read confirmation from")
	}
	var line string
	if _, err := fmt.Fscanln(stdin, &line); err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if requireKeyword != "" {
		if strings.TrimSpace(line) != requireKeyword {
			return fmt.Errorf("aborted: expected %q, got %q", requireKeyword, line)
		}
		return nil
	}
	resp := strings.ToLower(strings.TrimSpace(line))
	if resp == "y" || resp == "yes" {
		return nil
	}
	return fmt.Errorf("aborted: confirmation declined")
}
