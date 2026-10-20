package ui

import (
	"strings"
	"time"
)

func bashHeader(t Turn, w int, opts JSOpts) Line {
	title := t.Title
	if title == "" {
		title = "Bash command"
	}
	elapsed := "?"
	if t.ElapsedMs > 0 {
		elapsed = FmtDur(time.Duration(t.ElapsedMs) * time.Millisecond)
	}
	if opts.Running {
		elapsed = FmtDur(opts.Elapsed)
	}
	// The rule glyphs may use the gutter; the title starts at column Gutter and
	// the elapsed time ends before the right gutter.
	if w < TextWidth(elapsed)+2*Gutter+2 {
		return Line{Segs: []Seg{{Text: Truncate(elapsed, max(0, w-2*Gutter)), FG: FgFaint}}, BG: BgTerm}
	}
	title = Truncate(title, max(0, w-TextWidth(elapsed)-6))
	gap := max(0, w-TextWidth(title)-TextWidth(elapsed)-6)
	text := "─ " + title + " " + strings.Repeat("─", gap) + " " + elapsed + " ─"
	return Line{Segs: []Seg{{Text: Truncate(text, w), FG: FgFaint}}, BG: BgTerm, FullWidth: true}
}
