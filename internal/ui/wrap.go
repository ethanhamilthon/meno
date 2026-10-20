package ui

import (
	"strings"

	"github.com/rivo/uniseg"
)

// wcell is one grapheme of styled text with its terminal width.
type wcell struct {
	g     string
	w     int
	seg   Seg // the style; Text is empty
	space bool
	nl    bool
}

// cellsOf splits segments into cells. A newline becomes a hard break, a tab one
// space; other control characters are dropped so that model output can never
// send escape sequences to the terminal.
func cellsOf(segs []Seg) []wcell { return styledCells(segs, false) }

func sourceCells(segs []Seg) []wcell { return styledCells(segs, true) }

func styledCells(segs []Seg, preserveTabs bool) []wcell {
	var out []wcell
	for _, s := range segs {
		style := s
		style.Text, style.glue = "", false
		g := uniseg.NewGraphemes(s.Text)
		for g.Next() {
			gr := g.Str()
			if gr == "\n" || gr == "\r\n" {
				out = append(out, wcell{nl: true, seg: style})
				continue
			}
			r := firstRune(gr)
			switch {
			case r == '\t':
				if preserveTabs {
					out = append(out, wcell{g: gr, w: 4, seg: style, space: true})
					continue
				}
				gr = " "
			case r < ' ' || r == 0x7f || (r >= 0x80 && r < 0xa0):
				continue
			}
			out = append(out, wcell{g: gr, w: uniseg.StringWidth(gr), seg: style, space: gr == " " && !s.glue})
		}
	}
	return out
}

// joinCells merges neighbouring cells of the same style back into segments.
func joinCells(cells []wcell) []Seg {
	var segs []Seg
	for _, c := range cells {
		if n := len(segs); n > 0 {
			last := segs[n-1]
			last.Text = ""
			if last == c.seg {
				segs[n-1].Text += c.g
				continue
			}
		}
		s := c.seg
		s.Text = c.g
		segs = append(segs, s)
	}
	return segs
}

// breakCells lays cells out in lines of at most width columns, breaking at
// spaces and between wide (CJK) characters. A word longer than a line is split.
// Spaces at the start and end of a line are dropped. Without text there are no
// lines.
func breakCells(cells []wcell, width int) [][]wcell {
	width = max(1, width)
	var lines [][]wcell
	var cur, pend []wcell
	col, pendW := 0, 0
	flush := func() {
		lines = append(lines, cur)
		cur, pend, col, pendW = nil, nil, 0, 0
	}
	for i := 0; i < len(cells); {
		c := cells[i]
		switch {
		case c.nl:
			flush()
			i++
		case c.space:
			if col > 0 {
				pend = append(pend, c)
				pendW += c.w
			}
			i++
		default:
			j := i + 1
			if c.w < 2 {
				for j < len(cells) && !cells[j].nl && !cells[j].space && cells[j].w < 2 {
					j++
				}
			}
			word := cells[i:j]
			ww := 0
			for _, wc := range word {
				ww += wc.w
			}
			if col > 0 && col+pendW+ww > width {
				flush()
			}
			if col > 0 {
				cur = append(cur, pend...)
				col += pendW
			}
			pend, pendW = nil, 0
			if ww <= width-col {
				cur = append(cur, word...)
				col += ww
			} else {
				for _, wc := range word {
					if col+wc.w > width && col > 0 {
						lines = append(lines, cur)
						cur, col = nil, 0
					}
					cur = append(cur, wc)
					col += wc.w
				}
			}
			i = j
		}
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// wrapWords breaks styled text into lines at word boundaries.
func wrapWords(segs []Seg, width int) []Line {
	var out []Line
	for _, l := range breakCells(cellsOf(segs), width) {
		out = append(out, Line{Segs: joinCells(l), BG: BgTerm})
	}
	return out
}

func segsWidth(segs []Seg) int {
	n := 0
	for _, s := range segs {
		n += TextWidth(s.Text)
	}
	return n
}

// clipSegs cuts segments after width columns.
func clipSegs(segs []Seg, width int) []Seg {
	var out []Seg
	col := 0
	for _, s := range segs {
		var b strings.Builder
		g := uniseg.NewGraphemes(s.Text)
		for g.Next() {
			w := uniseg.StringWidth(g.Str())
			if col+w > width {
				break
			}
			b.WriteString(g.Str())
			col += w
		}
		if b.Len() > 0 {
			s.Text = b.String()
			out = append(out, s)
		}
		if col >= width {
			break
		}
	}
	return out
}

// fitRow clips segments to width and pads them with background bg up to it.
func fitRow(segs []Seg, width int, fg Seg) []Seg {
	segs = clipSegs(segs, width)
	if pad := width - segsWidth(segs); pad > 0 {
		fg.Text = strings.Repeat(" ", pad)
		segs = append(segs, fg)
	}
	return segs
}
