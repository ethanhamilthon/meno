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

// Text is the plain text of the line (what the user sees, without the left padding).
func (l Line) Text() string {
	var b strings.Builder
	for _, s := range l.Segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

// PadX is the left padding DrawLine puts before the first segment.
const PadX = padX

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

// CodeLines highlights code and soft-wraps rows longer than width (width <= 0
// disables wrapping). max limits the number of rendered rows (0 = all).
func CodeLines(code string, bg tcell.Color, width, max int) []Line {
	var out []Line
	for _, src := range strings.Split(code, "\n") {
		segs := HighlightJS(src)
		rows := []Line{{Segs: segs, BG: bg}}
		if width > 0 {
			rows = wrapSegments(segs, width)
		}
		for _, r := range rows {
			if max > 0 && len(out) == max {
				return append(out, Line{Segs: []Seg{{Text: "  …", FG: FgDim}}, BG: bg})
			}
			r.BG = bg
			out = append(out, r)
		}
	}
	return out
}
