package app

// rng is a normalized operator range over a document: flat rune offsets
// [lo, hi); linewise ranges cover whole lines (hi excludes the last newline).
type rng struct {
	lo, hi int
	line   bool
}

// span turns a motion result into a range. a and b may be in any order.
func (t vtext) span(a, b vpos, k mkind) rng {
	if b.less(a) {
		a, b = b, a
	}
	switch k {
	case mLine:
		return rng{t.idx(vpos{a.line, 0}), t.idx(vpos{b.line, len(t[b.line])}), true}
	case mIncl:
		return rng{t.idx(a), min(t.idx(b)+1, len(t.flat())), false}
	}
	// An exclusive motion ending in column 0 stops at the end of the previous line.
	if b.col == 0 && b.line > a.line {
		b = vpos{b.line - 1, len(t[b.line-1])}
	}
	return rng{t.idx(a), t.idx(b), false}
}

func (t vtext) slice(r rng) string { return string(t.flat()[r.lo:r.hi]) }

// cut removes r and returns the new document and where the cursor goes.
func (t vtext) cut(r rng) (vtext, vpos) {
	f := t.flat()
	lo, hi := r.lo, r.hi
	if r.line {
		if hi < len(f) {
			hi++
		} else if lo > 0 {
			lo--
		}
	}
	out := append(append([]rune(nil), f[:lo]...), f[hi:]...)
	nt := splitRunes(out)
	p := nt.pos(min(lo, len(out)))
	if r.line {
		p = nt.clamp(vpos{nt.pos(min(r.lo, len(out))).line, 0}, false)
		p.col = firstNonBlank(nt[p.line])
	}
	return nt, p
}

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

// put inserts text from a register. Linewise text goes on its own lines
// below (or above) the cursor line; charwise text after (or at) the cursor.
func (t vtext) put(p vpos, text string, linewise, before bool) (vtext, vpos) {
	if text == "" {
		return t, p
	}
	if linewise {
		lines := splitRunes([]rune(text))
		at := p.line
		if !before {
			at++
		}
		out := append(append(append(vtext(nil), t[:at]...), lines...), t[at:]...)
		return out, vpos{at, firstNonBlank(lines[0])}
	}
	f := t.flat()
	i := t.idx(p)
	if !before && len(t[p.line]) > 0 {
		i++
	}
	ins := []rune(text)
	out := append(append(append([]rune(nil), f[:i]...), ins...), f[i:]...)
	nt := splitRunes(out)
	if len(splitRunes(ins)) > 1 {
		return nt, nt.pos(i)
	}
	return nt, nt.pos(i + len(ins) - 1)
}

// joinLines joins n lines starting at line with single spaces (vim J).
func (t vtext) joinLines(line, n int) (vtext, vpos) {
	end := min(line+max(2, n), len(t))
	if end-line < 2 {
		return t, vpos{line, t.maxCol(line, false)}
	}
	cur := append([]rune(nil), t[line]...)
	col := max(0, len(cur)-1)
	for _, l := range t[line+1 : end] {
		l = []rune(trimLeftSpace(string(l)))
		col = len(cur)
		if len(l) > 0 && len(cur) > 0 && cur[len(cur)-1] != ' ' && l[0] != ')' {
			cur = append(cur, ' ') // the cursor lands on the inserted space
		}
		if len(l) == 0 {
			col = max(0, len(cur)-1)
		}
		cur = append(cur, l...)
	}
	out := append(append(append(vtext(nil), t[:line]...), cur), t[end:]...)
	return out, vpos{line, col}
}

func trimLeftSpace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[i:]
		}
	}
	return ""
}
