package render

import (
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

func joinSegs(s []Seg) string {
	out := ""
	for _, x := range s {
		out += x.Text
	}
	return out
}
