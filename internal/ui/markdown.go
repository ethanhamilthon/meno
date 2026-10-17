package ui

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownLines renders Markdown directly into styled terminal cells (no ANSI).
func MarkdownLines(src string, width int, glyph string) []Line {
	source := []byte(src)
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
	var out []Line
	var segs []Seg
	flush := func() {
		if len(segs) > 0 {
			out = append(out, wrapSegments(segs, max(1, width))...)
			segs = nil
		}
	}
	var visit func(ast.Node, bool)
	visit = func(n ast.Node, bold bool) {
		switch v := n.(type) {
		case *ast.Text:
			segs = append(segs, Seg{Text: string(v.Segment.Value(source)), FG: FgWhite, Bold: bold})
			if v.HardLineBreak() || v.SoftLineBreak() {
				segs = append(segs, Seg{Text: " ", FG: FgWhite})
			}
			return
		case *ast.CodeSpan:
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					segs = append(segs, Seg{Text: string(t.Segment.Value(source)), FG: Blue})
				}
			}
			return
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			flush()
			for i := 0; i < n.Lines().Len(); i++ {
				l := n.Lines().At(i)
				out = append(out, wrapSegments([]Seg{{Text: strings.TrimRight(string(l.Value(source)), "\n"), FG: Blue}}, max(1, width))...)
			}
			return
		case *ast.Heading:
			flush()
			bold = true
		case *ast.Emphasis:
			bold = true
		case *ast.ListItem:
			flush()
			segs = append(segs, Seg{Text: "• ", FG: Purple})
		case *ast.Blockquote:
			flush()
			segs = append(segs, Seg{Text: "│ ", FG: FgDim})
		case *ast.ThematicBreak:
			flush()
			out = append(out, Plain(strings.Repeat("─", max(1, min(width, 40))), FgDim, BgTerm))
			return
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			visit(c, bold)
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.TextBlock, *ast.Heading, *ast.ListItem:
			flush()
		}
	}
	visit(doc, false)
	flush()
	for i := range out {
		mark := "  "
		if i == 0 {
			mark = glyph + " "
		}
		out[i].Segs = append([]Seg{{Text: mark, FG: Purple}}, out[i].Segs...)
	}
	return out
}

func wrapSegments(segs []Seg, width int) []Line {
	out := []Line{}
	line := Line{BG: BgTerm}
	col := 0
	for _, s := range segs {
		g := uniseg.NewGraphemes(s.Text)
		for g.Next() {
			gr := g.Str()
			w := uniseg.StringWidth(gr)
			if col+w > width && col > 0 {
				out = append(out, line)
				line = Line{BG: BgTerm}
				col = 0
			}
			if len(line.Segs) > 0 && line.Segs[len(line.Segs)-1].FG == s.FG && line.Segs[len(line.Segs)-1].Bold == s.Bold {
				line.Segs[len(line.Segs)-1].Text += gr
			} else {
				t := s
				t.Text = gr
				line.Segs = append(line.Segs, t)
			}
			col += w
		}
	}
	return append(out, line)
}
