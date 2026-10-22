package render

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

func DrawLine(c Canvas, x, y, w int, l Line) {
	c.Fill(x, y, w, 1, l.BG)
	g := min(Gutter, max(0, (w-1)/2))
	cx := x + g
	hi := x + w - g
	if l.FullWidth {
		cx = x
	}
	for _, s := range l.Segs {
		cx += c.Text(cx, y, max(0, hi-cx), s.Text, s.style(l.BG))
	}
	if aw := segsWidth(l.Aside); aw > 0 {
		ax := x + w - padX - aw
		if ax > cx {
			for _, s := range l.Aside {
				ax += c.Text(ax, y, x+w-ax, s.Text, s.style(l.BG))
			}
		}
	}
}

// style is the cell style of the segment on a line with background bg.
func (s Seg) style(bg tcell.Color) tcell.Style {
	if s.BG != 0 {
		bg = s.BG
	}
	st := tcell.StyleDefault.Foreground(s.FG).Background(bg)
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Under {
		st = st.Underline(true)
	}
	if s.Strike {
		st = st.StrikeThrough(true)
	}
	return st
}

// DrawBar draws a one-row bar. Text stays inside the gutter on both sides:
// left text starts at x+Gutter, right text ends at x+w-Gutter. The center is
// optional and dropped when it does not fit.
func DrawBar(c Canvas, x, y, w int, bg tcell.Color, left, mid, right []Seg) {
	c.Fill(x, y, w, 1, bg)
	g := min(Gutter, max(0, (w-1)/2))
	lo, hi := x+g, x+w-g
	cx := lo
	for _, s := range left {
		cx += c.Text(cx, y, max(0, hi-cx), s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
	rw := TextWidth(joinSegs(right))
	// Keep both edge anchors visible on narrow terminals. The center is optional.
	rightW := max(0, hi-cx-1)
	if rw > rightW {
		right = []Seg{{Text: Truncate(joinSegs(right), rightW), FG: FgDim}}
		rw = TextWidth(joinSegs(right))
	}
	rx := hi - rw
	mw := TextWidth(joinSegs(mid))
	mx := x + (w-mw)/2
	if mx < cx+1 || mx+mw > rx-1 {
		mid = nil
	}
	cx = mx
	for _, s := range mid {
		cx += c.Text(cx, y, max(0, rx-1-cx), s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
	cx = rx
	for _, s := range right {
		cx += c.Text(cx, y, max(0, hi-cx), s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
}

func joinSegs(s []Seg) string {
	out := ""
	for _, x := range s {
		out += x.Text
	}
	return out
}

// Badge paints a colored label from the left edge. The color block may use the
// gutter; the label text starts at column Gutter.
func Badge(c Canvas, x, y int, label string, bg tcell.Color) int {
	st := tcell.StyleDefault.Foreground(FgBlack).Background(bg)
	lead := strings.Repeat(" ", Gutter)
	return c.Text(x, y, 1000, lead+label+" ", st)
}
