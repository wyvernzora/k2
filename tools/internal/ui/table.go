package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

// Table prints a styled table or plain row lines.
func (r *Reporter) Table(headers []string, rows [][]string) {
	if r == nil || r.out == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		r.writePlainRows(rows)
		return
	}

	stateCol := findStateColumn(headers)
	displayHeaders := renderDisplayHeaders(headers)
	displayRows := renderDisplayRows(rows, stateCol)
	widths := computeVisualWidths(displayHeaders, displayRows)

	writeTableRow(r.out, displayHeaders, widths)
	writeTableSeparator(r.out, widths)
	for _, row := range displayRows {
		writeTableRow(r.out, row, widths)
	}
}

func (r *Reporter) writePlainRows(rows [][]string) {
	for _, row := range rows {
		fmt.Fprintf(r.out, "k2-tools: row: %s\n", strings.Join(row, " | "))
	}
}

func findStateColumn(headers []string) int {
	for i, h := range headers {
		low := strings.ToLower(h)
		if strings.Contains(low, "state") || strings.Contains(low, "status") {
			return i
		}
	}
	return -1
}

func renderDisplayHeaders(headers []string) []string {
	display := make([]string, len(headers))
	for i, h := range headers {
		display[i] = KeyStyle.Render(h)
	}
	return display
}

func renderDisplayRows(rows [][]string, stateCol int) [][]string {
	display := make([][]string, len(rows))
	for ri, row := range rows {
		out := make([]string, len(row))
		for ci, cell := range row {
			if ci == stateCol {
				out[ci] = renderStateCell(cell)
			} else {
				out[ci] = cell
			}
		}
		display[ri] = out
	}
	return display
}

func writeTableRow(out io.Writer, cells []string, widths []int) {
	fmt.Fprint(out, "  ")
	for i, cell := range cells {
		if i >= len(widths) {
			continue
		}
		fmt.Fprintf(out, "%s  ", padVisual(cell, widths[i]))
	}
	fmt.Fprintln(out)
}

func writeTableSeparator(out io.Writer, widths []int) {
	fmt.Fprint(out, "  ")
	for _, wd := range widths {
		fmt.Fprintf(out, "%s  ", AccentBoldStyle.Render(strings.Repeat("─", wd)))
	}
	fmt.Fprintln(out)
}

func renderStateCell(value string) string {
	low := strings.ToLower(strings.TrimSpace(value))
	if low == "running" || low == "active" || low == "ok" || low == "open" {
		return OkStyle.Render("●") + " " + value
	}
	if value == "" || low == "-" || low == "—" {
		return value
	}
	return DimStyle.Render("○") + " " + value
}

func computeVisualWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if wd := lipgloss.Width(cell); wd > widths[i] {
					widths[i] = wd
				}
			}
		}
	}
	return widths
}

func padVisual(s string, width int) string {
	cur := lipgloss.Width(s)
	if cur >= width {
		return s
	}
	return s + strings.Repeat(" ", width-cur)
}
