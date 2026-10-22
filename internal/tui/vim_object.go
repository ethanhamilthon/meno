package tui

// object resolves a text object (key like "iw", "a\"", "i(") around p.
// The range may be empty (e.g. inside "()"). ok is false when nothing matches.
func (t vtext) object(p vpos, key string) (rng, bool) {
	if len(t) == 0 || len(key) < 2 {
		return rng{}, false
	}
	p = t.clamp(p, false)
	around := key[0] == 'a'
	switch c := []rune(key)[1]; c {
	case 'w', 'W':
		return t.wordObject(p, around, c == 'W')
	case '"', '\'', '`':
		return t.quoteObject(p, around, c)
	case '(', ')', 'b':
		return t.pairObject(p, around, '(', ')')
	case '[', ']':
		return t.pairObject(p, around, '[', ']')
	case '{', '}', 'B':
		return t.pairObject(p, around, '{', '}')
	case '<', '>':
		return t.pairObject(p, around, '<', '>')
	case 'p':
		return t.paragraphObject(p, around)
	}
	return rng{}, false
}

func (t vtext) wordObject(p vpos, around, big bool) (rng, bool) {
	l := t[p.line]
	if len(l) == 0 {
		return rng{}, false
	}
	run := func(i int) (int, int) {
		c := class(l[i], big)
		s, e := i, i
		for s > 0 && class(l[s-1], big) == c {
			s--
		}
		for e+1 < len(l) && class(l[e+1], big) == c {
			e++
		}
		return s, e
	}
	s, e := run(p.col)
	if around {
		if class(l[p.col], big) == clsSpace { // whitespace plus the word after it
			if e+1 < len(l) {
				_, e = run(e + 1)
			}
		} else if e+1 < len(l) && class(l[e+1], big) == clsSpace {
			_, e = run(e + 1)
		} else if s > 0 && class(l[s-1], big) == clsSpace { // no trailing space: take leading
			s, _ = run(s - 1)
		}
	}
	base := t.idx(vpos{p.line, 0})
	return rng{base + s, base + e + 1, false}, true
}

func (t vtext) quoteObject(p vpos, around bool, q rune) (rng, bool) {
	l := t[p.line]
	var qs []int
	for i, r := range l {
		if r == q {
			qs = append(qs, i)
		}
	}
	for i := 0; i+1 < len(qs); i += 2 {
		s, e := qs[i], qs[i+1]
		if p.col <= e {
			base := t.idx(vpos{p.line, 0})
			if around {
				return rng{base + s, base + e + 1, false}, true
			}
			return rng{base + s + 1, base + e, false}, true
		}
	}
	return rng{}, false
}

func (t vtext) pairObject(p vpos, around bool, open, close rune) (rng, bool) {
	f := t.flat()
	i := t.idx(p)
	s := -1
	depth := 0
	for k := i; k >= 0; k-- {
		switch {
		case f[k] == close && k != i:
			depth++
		case f[k] == open:
			if depth == 0 {
				s = k
			} else {
				depth--
			}
		}
		if s >= 0 {
			break
		}
	}
	if s < 0 {
		return rng{}, false
	}
	e, depth := -1, 0
	for k := s + 1; k < len(f); k++ {
		if f[k] == open {
			depth++
		} else if f[k] == close {
			if depth == 0 {
				e = k
				break
			}
			depth--
		}
	}
	if e < 0 {
		return rng{}, false
	}
	if around {
		return rng{s, e + 1, false}, true
	}
	return rng{s + 1, e, false}, true
}

func (t vtext) paragraphObject(p vpos, around bool) (rng, bool) {
	blank := func(l int) bool { return len(t[l]) == 0 }
	s, e := p.line, p.line
	b := blank(p.line)
	for s > 0 && blank(s-1) == b {
		s--
	}
	for e+1 < len(t) && blank(e+1) == b {
		e++
	}
	if around && e+1 < len(t) {
		nb := blank(e + 1)
		e++
		for e+1 < len(t) && blank(e+1) == nb {
			e++
		}
	}
	return rng{t.idx(vpos{s, 0}), t.idx(vpos{e, len(t[e])}), true}, true
}
