package ui

import (
	"fmt"
	"strings"
)

// BannerKind selects the badge and text styling of a final summary.
type BannerKind int

const (
	BannerSuccess BannerKind = iota
	BannerWarn
	BannerFail
)

// Banner prints a final multi-line summary.
func (r *Reporter) Banner(kind BannerKind, lines ...string) {
	if r == nil || r.out == nil || len(lines) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		for _, line := range lines {
			fmt.Fprintf(r.out, "k2-tools: banner: %s\n", line)
		}
		return
	}
	var badgeKind BadgeKind
	switch kind {
	case BannerWarn:
		badgeKind = BadgeWarnKind
	case BannerFail:
		badgeKind = BadgeFailKind
	default:
		badgeKind = BadgeOkKind
	}
	rule := DimStyle.Render(strings.Repeat("─", 54))
	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "  %s\n", rule)
	fmt.Fprintf(r.out, "  %s  %s\n", RenderBadge(badgeKind), AccentBoldStyle.Render(lines[0]))
	for _, line := range lines[1:] {
		fmt.Fprintf(r.out, "        %s\n", DimStyle.Render(line))
	}
	fmt.Fprintf(r.out, "  %s\n", rule)
}
