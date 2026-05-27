package ui

import (
	"fmt"
	"strings"
)

// Section prints an inverse-video magenta phase header.
func (r *Reporter) Section(label string) {
	if r == nil || r.out == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		fmt.Fprintf(r.out, "k2-tools: == %s ==\n", label)
		return
	}
	bar := SectionBar.Render("▌ " + strings.ToUpper(label))
	rule := DimStyle.Render(strings.Repeat("─", 60))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "%s %s\n", bar, rule)
}
