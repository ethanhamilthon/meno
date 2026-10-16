package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Seg struct {
	Text string
	FG   tcell.Color
	Bold bool
}

type Line struct {
	Segs []Seg
	BG   tcell.Color
}

func (l Line) Width() int {
	n := 0
	for _, s := range l.Segs {
		n += TextWidth(s.Text)
	}
	return n
}

func Plain(text string, fg, bg tcell.Color) Line {
	return Line{Segs: []Seg{{Text: text, FG: fg}}, BG: bg}
}

func Band(bg tcell.Color) Line { return Line{BG: bg} }

func Glyphed(glyph string, glyphFG tcell.Color, text string, fg, bg tcell.Color) Line {
	return Line{Segs: []Seg{{Text: glyph + " ", FG: glyphFG}, {Text: text, FG: fg}}, BG: bg}
}

func WrapLines(text string, width int, mk func(string) Line) []Line {
	var out []Line
	for _, l := range Wrap(strings.TrimRight(text, "\n"), width) {
		out = append(out, mk(l))
	}
	return out
}

func CodeLines(code string, bg tcell.Color, max int) []Line {
	var out []Line
	src := strings.Split(code, "\n")
	for i, l := range src {
		if max > 0 && i == max {
			out = append(out, Line{Segs: []Seg{{Text: "  …", FG: FgDim}}, BG: bg})
			break
		}
		out = append(out, Line{Segs: HighlightJS(l), BG: bg})
	}
	return out
}
