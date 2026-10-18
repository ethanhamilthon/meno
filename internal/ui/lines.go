package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Seg struct {
	Text string
	FG   tcell.Color
	Bold bool
	// Italic, Under and Strike are text attributes. BG, when set, paints the
	// segment over the line's own background (inline code, code blocks).
	Italic, Under, Strike bool
	BG                    tcell.Color
	// glue marks padding spaces that belong to the word next to them: wrapping
	// must not break there or drop them.
	glue bool
}

type Line struct {
	Segs []Seg
	BG   tcell.Color
	// Aside is drawn at the right edge of the row, over free space only (the
	// timing badge of a js block). It is not part of the text of the line.
	Aside []Seg
	// Image is set on the first of the rows reserved for an inline picture.
	Image *ImageRef
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
	return CodeLinesOpt(code, bg, width, max, CodeOpts{})
}
