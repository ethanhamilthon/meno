package render

import (
	"github.com/gdamore/tcell/v2"
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
	// A failed call is told by the color of its time, not by a background.
	timeFG := FgFaint
	if t.Err != "" || t.TimedOut {
		timeFG = Red
	}
	return ruleLine(title, elapsed, timeFG, w)
}

// ruleLine is "─── title ───────── time": a full-width row with a title on the
// left and a time on the right.
func ruleLine(title, elapsed string, timeFG tcell.Color, w int) Line {
	// The title shares the body anchor; only the rule uses the outer gutter.
	// The right gutter is excluded even though this row is drawn full-width.
	available := max(0, w-Gutter)
	lead := "─── "
	if available < TextWidth(lead)+TextWidth(elapsed)+1 {
		return Line{Segs: []Seg{{Text: "  " + Truncate(elapsed, TranscriptTextWidth(w)), FG: timeFG}}, BG: BgTerm}
	}
	title = Truncate(title, max(0, available-TextWidth(lead)-TextWidth(elapsed)-2))
	gap := max(0, available-TextWidth(lead)-TextWidth(title)-TextWidth(elapsed)-2)
	text := lead + title + " " + strings.Repeat("─", gap) + " "
	return Line{Segs: []Seg{{Text: text, FG: FgFaint}, {Text: elapsed, FG: timeFG}}, BG: BgTerm, FullWidth: true}
}
