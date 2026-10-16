package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type Canvas struct {
	S tcell.Screen
}

func (c Canvas) Fill(x, y, w, h int, bg tcell.Color) {
	st := tcell.StyleDefault.Background(bg)
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.S.SetContent(i, j, ' ', nil, st)
		}
	}
}

func (c Canvas) Text(x, y, maxW int, s string, st tcell.Style) int {
	col := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		if col >= maxW {
			break
		}
		w := runeWidth(g.Str())
		if col+w > maxW {
			break
		}
		c.S.SetContent(x+col, y, firstRune(g.Str()), nil, st)
		if w == 2 {
			c.S.SetContent(x+col+1, y, ' ', nil, st)
		}
		col += w
	}
	return col
}

func runeWidth(s string) int {
	return uniseg.StringWidth(s)
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ' '
}

func TextWidth(s string) int { return uniseg.StringWidth(s) }

func Wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for _, para := range splitLines(s) {
		cur := ""
		curW := 0
		g := uniseg.NewGraphemes(para)
		for g.Next() {
			gr := g.Str()
			w := uniseg.StringWidth(gr)
			if curW+w > width {
				lines = append(lines, cur)
				cur, curW = "", 0
			}
			cur += gr
			curW += w
		}
		lines = append(lines, cur)
	}
	return lines
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
