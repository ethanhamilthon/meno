package app

import (
	"strings"

	"meno/internal/ui"
)

// statusMid is the centre of the status bar as colored segments: the
// connection state when it is not live, otherwise the counters of all sessions.
func (a *App) statusMid() []ui.Seg {
	if c := a.connLabel(); c != "" {
		return []ui.Seg{{Text: c, FG: ui.Orange}}
	}
	return a.globalCounts()
}

// barSegsWidth is the width of segments laid side by side.
func barSegsWidth(segs []ui.Seg) int { return ui.TextWidth(joinSegText(segs)) }

// compactSegs shortens "2 running" to "2r" in every counter and keeps the
// color of each segment; separators are left as they are.
func compactSegs(segs []ui.Seg) []ui.Seg {
	out := make([]ui.Seg, len(segs))
	copy(out, segs)
	for i, s := range out {
		if f := strings.Fields(s.Text); len(f) == 2 && s.Text != " | " {
			out[i].Text = f[0] + strings.ToLower(f[1][:1])
		}
	}
	return out
}

// drawBarSegs paints segments from x on, each in its own color over the status
// bar background, and never past x+w.
func drawBarSegs(c ui.Canvas, x, y, w int, segs []ui.Seg) {
	end := x + w
	for _, s := range segs {
		sw := ui.TextWidth(s.Text)
		if x+sw > end {
			return
		}
		fg := s.FG
		if s.Text == " | " {
			fg = ui.FgDim
		}
		c.Text(x, y, sw, s.Text, ui.Style{FG: fg, BG: ui.BgTerm, Bold: s.Bold}.TCell())
		x += sw
	}
}
