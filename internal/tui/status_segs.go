package tui

import (
	"strings"

	"meno/internal/render"
)

// statusMid is the centre of the status bar as colored segments: the
// connection state when it is not live, otherwise the counters of all sessions.
func (a *App) statusMid() []render.Seg {
	if c := a.connLabel(); c != "" {
		return []render.Seg{{Text: c, FG: render.Orange}}
	}
	return a.globalCounts()
}

// barSegsWidth is the width of segments laid side by side.
func barSegsWidth(segs []render.Seg) int { return render.TextWidth(joinSegText(segs)) }

// compactSegs shortens "2 running" to "2r" in every counter and keeps the
// color of each segment; separators are left as they are.
func compactSegs(segs []render.Seg) []render.Seg {
	out := make([]render.Seg, len(segs))
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
func drawBarSegs(c render.Canvas, x, y, w int, segs []render.Seg) {
	end := x + w
	for _, s := range segs {
		sw := render.TextWidth(s.Text)
		if x+sw > end {
			return
		}
		fg := s.FG
		if s.Text == " | " {
			fg = render.FgDim
		}
		c.Text(x, y, sw, s.Text, render.Style{FG: fg, BG: render.BgTerm, Bold: s.Bold}.TCell())
		x += sw
	}
}
