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
	if w < TextWidth(elapsed)+5 {
		return Line{Segs: []Seg{{Text: Truncate(elapsed, max(0, w)), FG: FgDim}}, BG: BgTerm, FullWidth: true}
	}
	title = Truncate(title, max(0, w-TextWidth(elapsed)-5))
	gap := max(0, w-TextWidth(title)-TextWidth(elapsed)-5)
	text := "─ " + title + " " + strings.Repeat("─", gap) + " " + elapsed
	return Line{Segs: []Seg{{Text: Truncate(text, w), FG: FgDim}}, BG: BgTerm, FullWidth: true}
}
