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
	// FoldStrings hides the content of string literals beyond FoldLines lines
	// (0 hides it all but a marker). The source itself is never changed.
	FoldStrings bool
	FoldLines   int
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

// styledSegs turns tokens into coloured segments, folding string content.
func styledSegs(toks []jsTok, o CodeOpts) []Seg {
	var out []Seg
	add := func(text string, fg tcell.Color) {
		if text != "" {
			out = append(out, Seg{Text: text, FG: fg})
		}
	}
	marker := func() { add("…", FgDim) }
	// Remaining content lines of the template being shown.
	remaining, inTpl := 0, false
	for i, t := range toks {
		switch t.Kind {
		case jsString:
			if !o.FoldStrings {
				add(t.Text, Green)
				continue
			}
			q, body := t.Text[:1], t.Text[1:len(t.Text)-1]
			add(q, Green)
			foldInto(body, o.FoldLines, &remaining, true, add, marker)
			add(q, Green)
		case jsTplOpen:
			inTpl, remaining = true, o.FoldLines
			add(t.Text, Green)
		case jsTplText:
			if !o.FoldStrings || !inTpl {
				add(t.Text, Green)
				continue
			}
			foldInto(t.Text, o.FoldLines, &remaining, false, add, marker)
		case jsTplClose:
			inTpl = false
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

// foldInto adds text with at most *remaining lines kept (limit for a lone
// string); the rest becomes one muted marker.
func foldInto(text string, limit int, remaining *int, lone bool, add func(string, tcell.Color), marker func()) {
	if text == "" {
		return
	}
	if lone {
		*remaining = limit
	}
	lines := strings.Split(text, "\n")
	switch {
	case limit == 0:
		marker()
	case len(lines) <= *remaining:
		add(text, Green)
		*remaining -= len(lines)
	case *remaining > 0:
		add(strings.Join(lines[:*remaining], "\n"), Green)
		marker()
		*remaining = 0
	default:
		marker()
	}
}

// CodeLinesOpt is CodeLines with highlighting and folding options.
func CodeLinesOpt(code string, bg tcell.Color, width, max int, o CodeOpts) []Line {
	var segs []Seg
	if !o.Plain {
		if toks, ok := lexJS(code); ok {
			segs = styledSegs(toks, o)
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
