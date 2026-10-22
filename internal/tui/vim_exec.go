package tui

import (
	"strings"
	"unicode"
)

func (a *App) setReg(text string, line bool) {
	a.st.Reg, a.st.RegLine = text, line
	if text != "" {
		a.scr.SetClipboard([]byte(externalText([]rune(text))))
	}
}

func (a *App) wantCol(t vtext, p vpos) int {
	if a.st.WantSet {
		return a.st.WantCol
	}
	return p.col
}

// resolveMotion runs the motion of c from p. forOp relaxes cursor limits and
// applies vim's special cases for w and cw.
func (a *App) resolveMotion(t vtext, p vpos, c vcmd, forOp bool) (vpos, mkind, bool) {
	args := moveArgs{n: c.n, hasN: c.hasN, want: a.wantCol(t, p), ch: c.ch, pastEnd: forOp}
	key := c.key
	switch key {
	case ";", ",":
		if a.st.LastFind == "" {
			return p, mExcl, false
		}
		key, args.ch = a.st.LastFind, a.st.LastFindC
		if c.key == "," {
			key = swapCase(key)
		}
	case "f", "F", "t", "T":
		a.st.LastFind, a.st.LastFindC = key, c.ch
	}
	if forOp && c.op == 'c' && (key == "w" || key == "W") {
		f, i := t.flat(), t.idx(p)
		if i < len(f) && !unicode.IsSpace(f[i]) {
			end := i - 1
			for k := 0; k < args.n; k++ {
				end = wordEnd(f, max(0, end), key == "W")
			}
			return t.pos(end), mIncl, true
		}
	}
	to, kind, ok := t.motion(p, key, args)
	if ok && forOp && (key == "w" || key == "W") && to.line > p.line {
		// dw on the last word of a line stops at the end of that line
		to = vpos{to.line - 1, len(t[to.line-1])}
		if to.line < p.line {
			to = vpos{p.line, len(t[p.line])}
		}
	}
	return to, kind, ok
}

func swapCase(s string) string {
	if strings.ToUpper(s) == s {
		return strings.ToLower(s)
	}
	return strings.ToUpper(s)
}

// execNormal runs a complete normal-mode command on the input.
func (a *App) execNormal(c vcmd) {
	if a.st.InLog {
		a.execLogNormal(c)
		return
	}
	a.clampCursor()
	t := a.inputText()
	p := t.clamp(t.pos(a.st.Cursor), false)
	if c.op != 0 {
		a.execOperator(t, p, c)
		return
	}
	switch c.key {
	case "j", "k":
		a.moveVertical(c)
	case "h", "l", "0", "^", "$", "w", "W", "b", "B", "e", "E", "gg", "G", "{", "}", "f", "F", "t", "T", ";", ",":
		if to, _, ok := a.resolveMotion(t, p, c, false); ok {
			a.setCursorPos(t, t.clamp(to, false))
		}
	case "x", "X", "s", "D", "C", "S", "Y":
		m := map[string]vcmd{
			"x": {op: 'd', key: "l"}, "X": {op: 'd', key: "h"}, "s": {op: 'c', key: "l"},
			"D": {op: 'd', key: "$"}, "C": {op: 'c', key: "$"},
			"S": {op: 'c', key: "c", opLine: true}, "Y": {op: 'y', key: "y", opLine: true},
		}[c.key]
		m.n, m.hasN = c.n, c.hasN
		if c.key == "D" || c.key == "C" {
			m.n = c.n // $ with a count spans following lines
		}
		a.execOperator(t, p, m)
	case "i":
		a.beginInsert(a.snapshot())
	case "a":
		base := a.snapshot()
		if len(t[p.line]) > 0 {
			a.st.Cursor = t.idx(vpos{p.line, p.col + 1})
		}
		a.beginInsert(base)
	case "I":
		base := a.snapshot()
		a.st.Cursor = t.idx(vpos{p.line, firstNonBlankInsert(t[p.line])})
		a.beginInsert(base)
	case "A":
		base := a.snapshot()
		a.st.Cursor = t.idx(vpos{p.line, len(t[p.line])})
		a.beginInsert(base)
	case "o", "O":
		base := a.snapshot()
		at := p.line
		if c.key == "o" {
			at++
		}
		nt := append(append(append(vtext(nil), t[:at]...), nil), t[at:]...)
		a.setText(nt, vpos{at, 0})
		a.beginInsert(base)
	case "p", "P":
		if a.st.Reg == "" {
			// Nothing yanked: p takes an image from the system clipboard.
			if c.key == "p" && !a.st.InLog {
				a.attachFromClipboard(true)
			}
			return
		}
		a.pushUndo(a.snapshot())
		text := strings.Repeat(a.st.Reg, c.n)
		if a.st.RegLine {
			text = strings.TrimSuffix(strings.Repeat(a.st.Reg+"\n", c.n), "\n")
		}
		nt, np := t.put(p, text, a.st.RegLine, c.key == "P")
		a.setText(nt, nt.clamp(np, false))
	case "u":
		for i := 0; i < c.n; i++ {
			a.undo()
		}
	case "J":
		a.pushUndo(a.snapshot())
		nt, np := t.joinLines(p.line, max(2, c.n))
		a.setText(nt, np)
	case "r":
		if c.ch == 0 || p.col+c.n > len(t[p.line]) {
			return
		}
		a.pushUndo(a.snapshot())
		l := append([]rune(nil), t[p.line]...)
		for i := 0; i < c.n; i++ {
			l[p.col+i] = c.ch
		}
		nt := append(vtext(nil), t...)
		nt[p.line] = l
		a.setText(nt, vpos{p.line, p.col + c.n - 1})
	case "~":
		l := t[p.line]
		if len(l) == 0 {
			return
		}
		a.pushUndo(a.snapshot())
		nl := append([]rune(nil), l...)
		end := min(len(l), p.col+c.n)
		for i := p.col; i < end; i++ {
			nl[i] = toggleRune(nl[i])
		}
		nt := append(vtext(nil), t...)
		nt[p.line] = nl
		a.setText(nt, vpos{p.line, min(end, len(l)-1)})
	}
}

func firstNonBlankInsert(l []rune) int {
	for i, r := range l {
		if !unicode.IsSpace(r) {
			return i
		}
	}
	return len(l)
}

func toggleRune(r rune) rune {
	if unicode.IsUpper(r) {
		return unicode.ToLower(r)
	}
	return unicode.ToUpper(r)
}

// execOperator applies d / c / y to the range described by c.
func (a *App) execOperator(t vtext, p vpos, c vcmd) {
	var r rng
	if c.opLine {
		last := min(p.line+c.n-1, len(t)-1)
		r = t.span(p, vpos{last, 0}, mLine)
	} else if len(c.key) == 2 && (c.key[0] == 'i' || c.key[0] == 'a') {
		o, ok := t.object(p, c.key)
		if !ok {
			return
		}
		r = o
	} else {
		to, kind, ok := a.resolveMotion(t, p, c, true)
		if !ok {
			return
		}
		r = t.span(p, to, kind)
	}
	a.applyOp(t, c.op, r)
}

// applyOp runs operator op over r; the cursor is placed like vim does.
func (a *App) applyOp(t vtext, op rune, r rng) {
	text := t.slice(r)
	base := a.snapshot()
	switch op {
	case 'y':
		a.setReg(text, r.line)
		a.setCursorPos(t, t.clamp(t.pos(r.lo), false))
		if r.line {
			a.st.Cursor = base.cur
		}
	case 'd':
		if r.lo == r.hi && !r.line {
			return
		}
		a.setReg(text, r.line)
		a.pushUndo(base)
		nt, np := t.cut(r)
		a.setText(nt, nt.clamp(np, false))
	case 'c':
		a.setReg(text, r.line)
		if r.line {
			lo, hi := t.pos(r.lo), t.pos(r.hi)
			nt := append(append(append(vtext(nil), t[:lo.line]...), nil), t[hi.line+1:]...)
			a.setText(nt, vpos{lo.line, 0})
		} else {
			nt, np := t.cut(r)
			a.setText(nt, np)
		}
		a.beginInsert(base)
	}
}
