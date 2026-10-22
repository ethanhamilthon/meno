package render

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// CodeOpts says how to show tool-call source.
type CodeOpts struct {
	// Plain shows the source as readable unhighlighted text: for code that is
	// still streaming in, and the fallback for source that cannot be lexed.
	Plain bool
}

// CodeLinesOpt is CodeLines with highlighting and folding options.
func CodeLinesOpt(code string, bg tcell.Color, width, max int, o CodeOpts) []Line {
	var segs []Seg
	if !o.Plain {
		segs = bashSegments(code)
	}
	if segs == nil {
		segs = []Seg{{Text: code, FG: FgWhite}}
	}
	var lines [][]Seg
	cur := []Seg{}
	for _, s := range segs {
		parts := strings.Split(s.Text, "\n")
		for i, p := range parts {
			if i > 0 {
				lines = append(lines, cur)
				cur = []Seg{}
			}
			if p != "" {
				cur = append(cur, Seg{Text: p, FG: s.FG})
			}
		}
	}
	lines = append(lines, cur)
	var out []Line
	for _, ls := range lines {
		rows := []Line{{Segs: ls, BG: bg}}
		if width > 0 {
			rows = wrapSegments(ls, width)
		}
		for _, r := range rows {
			if max > 0 && len(out) == max {
				return append(out, Line{Segs: []Seg{{Text: "…", FG: FgDim}}, BG: bg})
			}
			r.BG = bg
			out = append(out, r)
		}
	}
	return out
}
