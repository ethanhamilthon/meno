package app

type mkind int

const (
	mExcl mkind = iota // operator range excludes the target
	mIncl              // includes the character under the target
	mLine              // whole lines
)

// moveArgs are the inputs of a motion besides the document and start position.
type moveArgs struct {
	n       int  // repeat count (>= 1)
	hasN    bool // an explicit count was typed (matters for G / gg)
	want    int  // sticky column for j / k
	ch      rune // argument of f F t T
	pastEnd bool // cursor may rest one past the last rune (insert-like positions)
}

// motion applies motion m (e.g. "w", "gg", "f") to p. ok is false when the
// motion does not exist or fails (e.g. f with no match).
func (t vtext) motion(p vpos, m string, a moveArgs) (vpos, mkind, bool) {
	a.n = max(1, a.n)
	if len(t) == 0 {
		return p, mExcl, false
	}
	p = t.clamp(p, true)
	line := t[p.line]
	switch m {
	case "h":
		return vpos{p.line, max(0, p.col-a.n)}, mExcl, true
	case "l":
		return vpos{p.line, min(p.col+a.n, t.maxCol(p.line, a.pastEnd))}, mExcl, true
	case "j", "k":
		d := a.n
		if m == "k" {
			d = -d
		}
		q := vpos{min(max(0, p.line+d), len(t)-1), 0}
		q.col = min(a.want, t.maxCol(q.line, a.pastEnd))
		return q, mLine, true
	case "0":
		return vpos{p.line, 0}, mExcl, true
	case "^":
		return vpos{p.line, firstNonBlank(line)}, mExcl, true
	case "$":
		l := min(p.line+a.n-1, len(t)-1)
		return vpos{l, t.maxCol(l, a.pastEnd)}, mIncl, true
	case "G", "gg":
		l := len(t) - 1
		if m == "gg" {
			l = 0
		}
		if a.hasN {
			l = min(a.n-1, len(t)-1)
		}
		return vpos{l, firstNonBlank(t[l])}, mLine, true
	case "w", "W", "e", "E", "b", "B":
		f := t.flat()
		i := t.idx(p)
		big := m == "W" || m == "E" || m == "B"
		for k := 0; k < a.n; k++ {
			switch m {
			case "w", "W":
				i = wordForward(f, i, big)
			case "e", "E":
				i = wordEnd(f, i, big)
			default:
				i = wordBack(f, i, big)
			}
		}
		kind := mExcl
		if m == "e" || m == "E" {
			kind = mIncl
		}
		return t.pos(i), kind, true
	case "}", "{":
		l, step := p.line, 1
		if m == "{" {
			step = -1
		}
		for k := 0; k < a.n; k++ {
			for l+step >= 0 && l+step < len(t) && len(t[l]) == 0 {
				l += step
			}
			for l+step >= 0 && l+step < len(t) && len(t[l]) != 0 {
				l += step
			}
		}
		if m == "}" && len(t[l]) != 0 {
			return vpos{l, len(t[l])}, mExcl, true
		}
		return vpos{l, 0}, mExcl, true
	case "f", "t", "F", "T":
		col, found := p.col, 0
		fwd := m == "f" || m == "t"
		for i := p.col; ; {
			if fwd {
				i++
			} else {
				i--
			}
			if i < 0 || i >= len(line) {
				break
			}
			if line[i] == a.ch {
				found++
				col = i
				if found == a.n {
					break
				}
			}
		}
		if found < a.n {
			return p, mExcl, false
		}
		switch m {
		case "t":
			col--
		case "T":
			col++
		}
		kind := mIncl
		if !fwd {
			kind = mExcl
		}
		return vpos{p.line, col}, kind, true
	}
	return p, mExcl, false
}
