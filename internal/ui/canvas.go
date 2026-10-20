package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"strings"
)

type Canvas struct {
	S tcell.Screen
}

// Dim darkens the whole rendered backdrop while preserving text and attributes.
// Dialogs draw their opaque surface after this pass.
func (c Canvas) Dim() {
	w, h := c.S.Size()
	darken := func(color, fallback tcell.Color) tcell.Color {
		if color == tcell.ColorDefault {
			color = fallback
		}
		r, g, b := color.RGB()
		return tcell.NewRGBColor(r/2, g/2, b/2)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; {
			r, combining, st, width := c.S.GetContent(x, y)
			fg, bg, _ := st.Decompose()
			c.S.SetContent(x, y, r, combining, st.Foreground(darken(fg, FgSoft)).Background(darken(bg, BgTerm)))
			x += max(1, width)
		}
	}
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
	s = strings.ReplaceAll(s, "\t", "    ")
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
		runes := []rune(g.Str())
		if len(runes) == 0 {
			continue
		}
		if runes[0] < ' ' || runes[0] == 127 {
			runes = []rune{' '}
			w = 1
		}
		c.S.SetContent(x+col, y, runes[0], runes[1:], st)
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
