package ui

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

func tokColor(k jsKind, text string, nextCall bool) tcell.Color {
	switch k {
	case jsKeyword:
		return Purple
	case jsLiteral:
		return Orange
	case jsNumber:
		return Orange
	case jsString, jsTplOpen, jsTplText, jsTplClose:
		return Green
	case jsRegex:
		return Red
	case jsComment:
		return FgDim
	case jsTplExpr:
		return Orange
	case jsIdent:
		if nextCall {
			return Blue
		}
	}
	return FgWhite
}

// styledSegs turns tokens into coloured segments.
func styledSegs(toks []jsTok) []Seg {
	var out []Seg
	add := func(text string, fg tcell.Color) {
		if text != "" {
			out = append(out, Seg{Text: text, FG: fg})
		}
	}
	for i, t := range toks {
		switch t.Kind {
		case jsString, jsTplOpen, jsTplText, jsTplClose:
			add(t.Text, Green)
		default:
			next := false
			if t.Kind == jsIdent {
				for j := i + 1; j < len(toks); j++ {
					if toks[j].Kind == jsSpace {
						continue
					}
					next = toks[j].Kind == jsPunct && toks[j].Text == "("
					break
				}
			}
			add(t.Text, tokColor(t.Kind, t.Text, next))
		}
	}
	return out
}

// CodeLinesOpt is CodeLines with highlighting and folding options.
func CodeLinesOpt(code string, bg tcell.Color, width, max int, o CodeOpts) []Line {
	var segs []Seg
	if !o.Plain {
		if toks, ok := lexJS(code); ok {
			segs = styledSegs(toks)
		}
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
				return append(out, Line{Segs: []Seg{{Text: "  …", FG: FgDim}}, BG: bg})
			}
			r.BG = bg
			out = append(out, r)
		}
	}
	return out
}
