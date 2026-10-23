package tui

// vpos is a position in a document made of lines; col is a rune index.
type vpos struct{ line, col int }

func (p vpos) less(q vpos) bool { return p.line < q.line || p.line == q.line && p.col < q.col }

// vtext is a line-based document: the common view over the timeline and the
// composer, so a mouse selection works the same in both.
type vtext [][]rune

// maxCol is the last valid column of a line; past the end is one more.
func (t vtext) maxCol(line int, pastEnd bool) int {
	n := len(t[line])
	if pastEnd {
		return n
	}
	return max(0, n-1)
}

func (t vtext) clamp(p vpos, pastEnd bool) vpos {
	if len(t) == 0 {
		return vpos{}
	}
	p.line = min(max(0, p.line), len(t)-1)
	p.col = min(max(0, p.col), t.maxCol(p.line, pastEnd))
	return p
}

// flat joins the lines with '\n'; idx/pos convert between the two views.
func (t vtext) flat() []rune {
	var out []rune
	for i, l := range t {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return out
}

func (t vtext) idx(p vpos) int {
	n := 0
	for i := 0; i < p.line && i < len(t); i++ {
		n += len(t[i]) + 1
	}
	return n + p.col
}

func (t vtext) pos(idx int) vpos {
	for i, l := range t {
		if idx <= len(l) {
			return vpos{i, idx}
		}
		idx -= len(l) + 1
	}
	if len(t) == 0 {
		return vpos{}
	}
	return vpos{len(t) - 1, len(t[len(t)-1])}
}

// rng is a normalized operator range over a document: flat rune offsets
// [lo, hi); linewise ranges cover whole lines (hi excludes the last newline).
type rng struct {
	lo, hi int
	line   bool
}

// span turns two positions into a range; a and b may be in any order. With
// line set it covers whole lines, otherwise both ends are included.
func (t vtext) span(a, b vpos, line bool) rng {
	if b.less(a) {
		a, b = b, a
	}
	if line {
		return rng{t.idx(vpos{a.line, 0}), t.idx(vpos{b.line, len(t[b.line])}), true}
	}
	return rng{t.idx(a), min(t.idx(b)+1, len(t.flat())), false}
}

func (t vtext) slice(r rng) string { return string(t.flat()[r.lo:r.hi]) }

func splitRunes(f []rune) vtext {
	t := vtext{nil}
	for _, r := range f {
		if r == '\n' {
			t = append(t, nil)
			continue
		}
		t[len(t)-1] = append(t[len(t)-1], r)
	}
	return t
}
