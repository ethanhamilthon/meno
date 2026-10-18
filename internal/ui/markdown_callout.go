package ui

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
)

// callout recognises a GitHub style alert: a quote whose first line is only
// [!NOTE] (or TIP, IMPORTANT, WARNING, CAUTION). It returns the upper case
// kind and registers where the first paragraph continues.
func (r *mdr) callout(q *ast.Blockquote) string {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok {
		return ""
	}
	var raw strings.Builder
	var rest ast.Node
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			return ""
		}
		raw.Write(t.Segment.Value(r.src))
		if t.SoftLineBreak() || t.HardLineBreak() {
			rest = c.NextSibling()
			break
		}
	}
	m := calloutTag.FindStringSubmatch(strings.TrimSpace(raw.String()))
	if m == nil {
		return ""
	}
	r.paraFrom[p] = rest
	return strings.ToUpper(m[1])
}

func (r *mdr) quote(q *ast.Blockquote, w int) []Line {
	kind := r.callout(q)
	saved := r.base
	bar := FgDim
	r.base.fg = FgSoft
	if kind == "" {
		r.base.italic = true
	} else {
		bar = calloutColor(kind)
	}
	body := r.children(q, max(1, w-2), false)
	r.base = saved
	var out []Line
	if kind != "" {
		out = append(out, Line{Segs: []Seg{{Text: "▎ ", FG: bar}, {Text: "● ", FG: bar}, {Text: kind, FG: bar, Bold: true}}, BG: BgTerm})
	}
	out = append(out, body...)
	if len(out) == 0 {
		out = []Line{{BG: BgTerm}}
	}
	for i := range out {
		if kind != "" && i == 0 {
			continue
		}
		out[i].Segs = append([]Seg{{Text: "▎ ", FG: bar}}, out[i].Segs...)
	}
	return out
}

// ---- inline ----

func (r *mdr) inlines(parent ast.Node, st mstyle) []Seg {
	return r.inlinesFrom(parent.FirstChild(), st)
}

func (r *mdr) inlinesFrom(c ast.Node, st mstyle) []Seg {
	var segs []Seg
	for ; c != nil; c = c.NextSibling() {
		r.inline(c, st, &segs)
	}
	return segs
}

func (r *mdr) inline(n ast.Node, st mstyle, out *[]Seg) {
	add := func(t string) {
		if t != "" {
			*out = append(*out, st.seg(t))
		}
	}
	switch v := n.(type) {
	case *ast.Text:
		add(string(v.Segment.Value(r.src)))
		if v.HardLineBreak() {
			add("\n")
		} else if v.SoftLineBreak() {
			// Agents write one thought per line: a single newline stays a line break.
			add("\n")
		}
	case *ast.String:
		add(string(v.Value))
	case *ast.CodeSpan:
		code := strings.ReplaceAll(r.plain(n), "\n", " ")
		cs := st
		cs.fg, cs.bg = Orange, BgPopup
		pad := cs.seg(" ")
		pad.glue = true
		*out = append(*out, pad, cs.seg(code), pad)
	case *ast.Emphasis:
		s := st
		if v.Level >= 2 {
			s.bold = true
		} else {
			s.italic = true
		}
		*out = append(*out, r.inlines(n, s)...)
	case *east.Strikethrough:
		s := st
		s.strike, s.fg = true, FgDim
		*out = append(*out, r.inlines(n, s)...)
	case *ast.Link:
		s := st
		s.fg, s.under = Blue, true
		label := r.inlines(n, s)
		dest := string(v.Destination)
		if segsWidth(label) == 0 {
			label = []Seg{s.seg(dest)}
		}
		*out = append(*out, label...)
		r.host(out, dest, joinText(label))
	case *ast.AutoLink:
		s := st
		s.fg, s.under = Blue, true
		*out = append(*out, s.seg(string(v.Label(r.src))))
	case *ast.Image:
		alt := strings.TrimSpace(r.plain(n))
		dest := string(v.Destination)
		if alt == "" {
			alt = path.Base(strings.TrimRight(dest, "/"))
		}
		*out = append(*out, Seg{Text: "▣ ", FG: FgDim})
		s := st
		s.fg, s.under = Blue, true
		*out = append(*out, s.seg(alt))
		r.host(out, dest, alt)
	case *ast.RawHTML:
		var raw strings.Builder
		for i := 0; i < v.Segments.Len(); i++ {
			seg := v.Segments.At(i)
			raw.Write(seg.Value(r.src))
		}
		tag := raw.String()
		switch {
		case strings.HasPrefix(tag, "<!--"):
		case brTag.MatchString(tag):
			add("\n")
		default:
			*out = append(*out, Seg{Text: tag, FG: FgDim})
		}
	case *east.TaskCheckBox:
	default:
		if n.HasChildren() {
			*out = append(*out, r.inlines(n, st)...)
		}
	}
}

var brTag = regexp.MustCompile("(?i)^<br\\s*/?>$")

// host appends " ↗ host" for a link to another site, unless the visible text
// already is the address.
func (r *mdr) host(out *[]Seg, dest, label string) {
	u, err := url.Parse(dest)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	bare := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(dest, "https://"), "http://"), "/")
	l := strings.TrimRight(strings.TrimSpace(label), "/")
	if l == bare || l == strings.TrimPrefix(bare, "www.") || l == dest || l == host {
		return
	}
	*out = append(*out, Seg{Text: " ↗ " + host, FG: FgDim})
}

func joinText(segs []Seg) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

// plain is the text inside a node, without styling.
func (r *mdr) plain(n ast.Node) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.Text:
				b.Write(v.Segment.Value(r.src))
				if v.SoftLineBreak() || v.HardLineBreak() {
					b.WriteByte(' ')
				}
			case *ast.String:
				b.Write(v.Value)
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}
