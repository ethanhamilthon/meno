package render

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
	// The title shares the body anchor; only the rule uses the outer gutter.
	// The right gutter is excluded even though this row is drawn full-width.
	available := max(0, w-Gutter)
	lead := "─── "
	if available < TextWidth(lead)+TextWidth(elapsed)+1 {
		return Line{Segs: []Seg{{Text: "  " + Truncate(elapsed, TranscriptTextWidth(w)), FG: FgFaint}}, BG: BgTerm}
	}
	title = Truncate(title, max(0, available-TextWidth(lead)-TextWidth(elapsed)-2))
	gap := max(0, available-TextWidth(lead)-TextWidth(title)-TextWidth(elapsed)-2)
	text := lead + title + " " + strings.Repeat("─", gap) + " " + elapsed
	return Line{Segs: []Seg{{Text: text, FG: FgFaint}}, BG: BgTerm, FullWidth: true}
}
