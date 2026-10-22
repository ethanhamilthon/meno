package tui

import "unicode"

// vpos is a position in a document made of lines; col is a rune index.
type vpos struct{ line, col int }

func (p vpos) less(q vpos) bool { return p.line < q.line || p.line == q.line && p.col < q.col }

// vtext is a line-based document. It is the common view over the input and the
// timeline so motions work the same in both.
type vtext [][]rune

// maxCol is the last valid column of a line; in normal mode the cursor sits on
// a character, in insert-like positions it may be one past the end.
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

const (
	clsSpace = iota
	clsWord
	clsPunct
)

func class(r rune, big bool) int {
	switch {
	case unicode.IsSpace(r):
		return clsSpace
	case big || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return clsWord
	}
	return clsPunct
}

// emptyLineAt reports whether flat[i] is a newline that ends an empty line.
func emptyLineAt(f []rune, i int) bool {
	return f[i] == '\n' && (i == 0 || f[i-1] == '\n')
}

// wordForward is vim's w / W; the result may be len(f) (end of text).
func wordForward(f []rune, i int, big bool) int {
	n := len(f)
	if i >= n {
		return n
	}
	if c := class(f[i], big); c != clsSpace {
		for i < n && class(f[i], big) == c {
			i++
		}
	} else if emptyLineAt(f, i) {
		i++
	}
	for i < n && class(f[i], big) == clsSpace && !emptyLineAt(f, i) {
		i++
	}
	return i
}

// wordEnd is vim's e / E.
func wordEnd(f []rune, i int, big bool) int {
	n := len(f)
	if i+1 >= n {
		return max(0, n-1)
	}
	i++
	for i < n && class(f[i], big) == clsSpace {
		i++
	}
	if i >= n {
		return n - 1
	}
	c := class(f[i], big)
	for i+1 < n && class(f[i+1], big) == c {
		i++
	}
	return i
}

// wordBack is vim's b / B.
func wordBack(f []rune, i int, big bool) int {
	if i <= 0 {
		return 0
	}
	i--
	for i > 0 && class(f[i], big) == clsSpace && !emptyLineAt(f, i) {
		i--
	}
	if class(f[i], big) == clsSpace {
		return i
	}
	c := class(f[i], big)
	for i > 0 && class(f[i-1], big) == c {
		i--
	}
	return i
}

// firstNonBlank is the column of the first non-space rune (or the last column).
func firstNonBlank(l []rune) int {
	for i, r := range l {
		if !unicode.IsSpace(r) {
			return i
		}
	}
	return max(0, len(l)-1)
}
