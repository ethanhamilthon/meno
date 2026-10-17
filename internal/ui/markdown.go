package ui

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var mdParser = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList, extension.Linkify)).Parser()

// mstyle is the style of inline text while walking the tree.
type mstyle struct {
	fg                          tcell.Color
	bold, italic, under, strike bool
	bg                          tcell.Color
}

func (s mstyle) seg(text string) Seg {
	return Seg{Text: text, FG: s.fg, Bold: s.bold, Italic: s.italic, Under: s.under, Strike: s.strike, BG: s.bg}
}

// mdr renders one document.
type mdr struct {
	src   []byte
	base  mstyle
	depth int // list nesting
	// paraFrom overrides where a paragraph starts rendering: a callout's first
	// line is its label and must not be repeated. A nil value means nothing is left.
	paraFrom map[ast.Node]ast.Node
}

// MarkdownLines renders Markdown directly into styled terminal cells (no ANSI):
// headings, emphasis, links, lists with task boxes, quotes and callouts, code
// blocks with syntax colours and diffs, and tables. width is the room for the
// text after the two-cell marker; glyph marks the first line.
func MarkdownLines(src string, width int, glyph string) []Line {
	src = strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n")
	source := []byte(src)
	doc := mdParser.Parse(text.NewReader(source))
	r := &mdr{src: source, base: mstyle{fg: FgWhite}, paraFrom: map[ast.Node]ast.Node{}}
	out := r.children(doc, max(1, width), false)
	for len(out) > 0 && len(out[len(out)-1].Segs) == 0 {
		out = out[:len(out)-1]
	}
	for i := range out {
		mark := "  "
		if i == 0 {
			mark = glyph + " "
		}
		out[i].Segs = append([]Seg{{Text: mark, FG: Purple}}, out[i].Segs...)
	}
	return out
}

func blank() Line { return Band(BgTerm) }

// children renders the blocks of a container, a blank line apart. In a tight
// list item the text and a nested list stay together.
func (r *mdr) children(parent ast.Node, w int, tight bool) []Line {
	var out []Line
	var prev ast.Node
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		lines := r.block(c, w)
		if len(lines) == 0 {
			continue
		}
		if len(out) > 0 && !(tight && tightPair(prev, c)) {
			out = append(out, blank())
		}
		out = append(out, lines...)
		prev = c
	}
	return out
}

func tightPair(prev, cur ast.Node) bool {
	_, pl := prev.(*ast.List)
	_, cl := cur.(*ast.List)
	_, pt := prev.(*ast.TextBlock)
	_, ct := cur.(*ast.TextBlock)
	return (pt || pl) && (cl || ct)
}

func (r *mdr) block(n ast.Node, w int) []Line {
	switch v := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		var segs []Seg
		if from, ok := r.paraFrom[n]; ok {
			segs = r.inlinesFrom(from, r.base)
		} else {
			segs = r.inlines(n, r.base)
		}
		return wrapWords(segs, w)
	case *ast.Heading:
		return r.heading(v, w)
	case *ast.List:
		return r.list(v, w)
	case *ast.Blockquote:
		return r.quote(v, w)
	case *ast.FencedCodeBlock:
		info := ""
		if v.Info != nil {
			info = string(v.Info.Segment.Value(r.src))
		}
		return codeBlock(info, r.lines(n), w)
	case *ast.CodeBlock:
		return codeBlock("", r.lines(n), w)
	case *ast.ThematicBreak:
		return []Line{Plain(strings.Repeat("─", max(1, min(w, 60))), FgDim, BgTerm)}
	case *ast.HTMLBlock:
		return r.htmlBlock(v, w)
	case *east.Table:
		return r.table(v, w)
	}
	if n.HasChildren() {
		return r.children(n, w, false)
	}
	return nil
}

// lines are the raw lines of a code block, without line ends.
func (r *mdr) lines(n ast.Node) []string {
	var out []string
	ls := n.Lines()
	for i := 0; i < ls.Len(); i++ {
		l := ls.At(i)
		out = append(out, strings.TrimRight(string(l.Value(r.src)), "\r\n"))
	}
	return out
}

func (r *mdr) heading(h *ast.Heading, w int) []Line {
	st := mstyle{fg: Blue, bold: true}
	switch {
	case h.Level == 1:
		st.fg = FgWhite
	case h.Level >= 3:
		st.fg = FgSoft
	}
	lines := wrapWords(r.inlines(h, st), w)
	if h.Level == 1 && len(lines) > 0 {
		rule := 0
		for _, l := range lines {
			rule = max(rule, l.Width())
		}
		lines = append(lines, Plain(strings.Repeat("━", max(1, min(rule, w))), FgDim, BgTerm))
	}
	return lines
}

func (r *mdr) htmlBlock(h *ast.HTMLBlock, w int) []Line {
	raw := strings.Join(r.lines(h), "\n")
	if t := strings.TrimSpace(raw); strings.HasPrefix(t, "<!--") && strings.HasSuffix(t, "-->") {
		return nil
	}
	if h.HasClosure() {
		raw += "\n" + string(h.ClosureLine.Value(r.src))
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return wrapWords([]Seg{{Text: strings.TrimRight(raw, "\r\n"), FG: FgDim}}, w)
}

var bullets = []string{"•", "◦", "▪"}

func (r *mdr) list(l *ast.List, w int) []Line {
	depth := r.depth
	r.depth++
	defer func() { r.depth = depth }()
	items := 0
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		items++
	}
	digits := len(strconv.Itoa(max(1, l.Start+items-1)))
	var out []Line
	i := 0
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		marker := Seg{Text: bullets[min(depth, len(bullets)-1)] + " ", FG: Purple}
		if l.IsOrdered() {
			n := strconv.Itoa(l.Start + i)
			marker = Seg{Text: strings.Repeat(" ", digits-len(n)) + n + ". ", FG: Purple, Bold: true}
		}
		saved := r.base
		if done, isTask := taskState(c); isTask {
			marker = Seg{Text: "[ ] ", FG: FgSoft}
			if done {
				marker = Seg{Text: "[x] ", FG: Green}
				r.base.fg = FgDim
			}
		}
		mw := TextWidth(marker.Text)
		body := r.children(c, max(1, w-mw), l.IsTight)
		r.base = saved
		if len(body) == 0 {
			body = []Line{{BG: BgTerm}}
		}
		for j := range body {
			lead := Seg{Text: strings.Repeat(" ", mw), FG: FgDim}
			if j == 0 {
				lead = marker
			}
			body[j].Segs = append([]Seg{lead}, body[j].Segs...)
		}
		if i > 0 && !l.IsTight {
			out = append(out, blank())
		}
		out = append(out, body...)
		i++
	}
	return out
}

// taskState reports whether a list item starts with a task box, and if it is ticked.
func taskState(item ast.Node) (done, task bool) {
	first := item.FirstChild()
	if first == nil {
		return false, false
	}
	if cb, ok := first.FirstChild().(*east.TaskCheckBox); ok {
		return cb.IsChecked, true
	}
	return false, false
}

var calloutTag = regexp.MustCompile("(?i)^\\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\\]$")

func calloutColor(kind string) tcell.Color {
	switch kind {
	case "TIP":
		return Green
	case "IMPORTANT":
		return Purple
	case "WARNING":
		return Yellow
	case "CAUTION":
		return Red
	}
	return Blue
}

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
