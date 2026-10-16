package ui

import "github.com/gdamore/tcell/v2"

func Row(c Canvas, x, y, w int, bg tcell.Color, fill bool) {
	if fill {
		c.Fill(x, y, w, 1, bg)
	}
}

func Chip(c Canvas, x, y int, label string, active bool) int {
	fg, bg := FgSoft, BgPopup
	if active {
		fg, bg = FgBlack, FgSoft
	}
	st := tcell.StyleDefault.Foreground(fg).Background(bg)
	w := c.Text(x, y, 1000, " "+label+" ", st)
	return w
}

func ChipActive(c Canvas, x, y int, label string, bg tcell.Color) int {
	st := tcell.StyleDefault.Foreground(FgBlack).Background(bg)
	return c.Text(x, y, 1000, " "+label+" ", st)
}

func Pad(s string, w int) string {
	n := TextWidth(s)
	if n >= w {
		return s
	}
	out := s
	for i := 0; i < w-n; i++ {
		out += " "
	}
	return out
}

func Truncate(s string, w int) string {
	if TextWidth(s) <= w {
		return s
	}
	if w <= 1 {
		return ""
	}
	r := []rune(s)
	for len(r) > 0 && TextWidth(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
