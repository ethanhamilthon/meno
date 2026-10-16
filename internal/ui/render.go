package ui

import (
	"github.com/gdamore/tcell/v2"
)

func DrawLine(c Canvas, x, y, w int, l Line) {
	c.Fill(x, y, w, 1, l.BG)
	cx := x + padX
	for _, s := range l.Segs {
		st := tcell.StyleDefault.Foreground(s.FG).Background(l.BG)
		if s.Bold {
			st = st.Bold(true)
		}
		cx += c.Text(cx, y, x+w-cx, s.Text, st)
	}
}

func DrawBar(c Canvas, x, y, w int, bg tcell.Color, left, mid, right []Seg) {
	c.Fill(x, y, w, 1, bg)
	cx := x
	for _, s := range left {
		cx += c.Text(cx, y, x+w-cx, s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
	rw := 0
	for _, s := range right {
		rw += TextWidth(s.Text)
	}
	rx := x + w - rw
	mx := x + (w-TextWidth(joinSegs(mid)))/2
	cx = mx
	for _, s := range mid {
		cx += c.Text(cx, y, w, s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
	cx = rx
	for _, s := range right {
		cx += c.Text(cx, y, x+w-cx, s.Text, tcell.StyleDefault.Foreground(s.FG).Background(bg))
	}
}

func joinSegs(s []Seg) string {
	out := ""
	for _, x := range s {
		out += x.Text
	}
	return out
}

func Badge(c Canvas, x, y int, label string, bg tcell.Color) int {
	return c.Text(x, y, 1000, " "+label+" ", tcell.StyleDefault.Foreground(FgBlack).Background(bg).Bold(true))
}
