package app

const readOnlyMsg = "the timeline is read-only · move to the input to edit"

// moveVertical moves j / k through the whole document, so k on the first input
// row enters the timeline and j on its last row comes back.
func (a *App) moveVertical(c vcmd) {
	t, top := a.doc()
	g := t.clamp(a.curG(top), true)
	if !a.st.WantSet {
		a.st.WantCol, a.st.WantSet = g.col, true
	}
	want := a.st.WantCol
	to, _, ok := t.motion(g, c.key, moveArgs{n: c.n, want: want})
	if !ok {
		return
	}
	a.setCurG(t.clamp(to, false), top)
	a.st.WantCol, a.st.WantSet = want, true
	a.followCursor()
}

// docRange resolves the operand of an operator (or a text object) on the whole
// document.
func (a *App) docRange(t vtext, top int, g vpos, c vcmd) (rng, bool) {
	switch {
	case c.opLine:
		return t.span(g, vpos{min(g.line+c.n-1, len(t)-1), 0}, mLine), true
	case len(c.key) == 2 && (c.key[0] == 'i' || c.key[0] == 'a'):
		return t.object(g, c.key)
	}
	to, kind, ok := a.resolveMotion(t, g, c, true)
	if !ok {
		return rng{}, false
	}
	return t.span(g, to, kind), true
}

// execLogNormal runs a normal-mode command while the cursor is in the
// timeline. Motions and yank work; editing jumps to the input or is refused.
func (a *App) execLogNormal(c vcmd) {
	t, top := a.doc()
	g := t.clamp(a.curG(top), false)
	if c.op != 0 {
		if c.op != 'y' {
			a.notifyErr(readOnlyMsg)
			return
		}
		r, ok := a.docRange(t, top, g, c)
		if !ok {
			return
		}
		a.setReg(t.slice(r), r.line)
		if !r.line {
			a.setCurG(t.clamp(t.pos(r.lo), false), top)
		}
		return
	}
	switch c.key {
	case "j", "k":
		a.moveVertical(c)
	case "gg", "G":
		line := 0
		if c.key == "G" {
			line = top - 1
		}
		if c.hasN {
			line = min(c.n-1, top-1)
		}
		a.setCurG(vpos{line, firstNonBlank(t[line])}, top)
		a.followCursor()
	case "h", "l", "0", "^", "$", "w", "W", "b", "B", "e", "E", "{", "}", "f", "F", "t", "T", ";", ",":
		if to, _, ok := a.resolveMotion(t, g, c, false); ok {
			a.setCurG(t.clamp(to, false), top)
			a.st.WantSet = false
			a.followCursor()
		}
	case "Y":
		a.execLogNormal(vcmd{n: c.n, hasN: c.hasN, op: 'y', key: "y", opLine: true})
	case "i", "a", "I", "A", "o", "O", "p", "P":
		a.st.InLog = false
		a.st.Cursor = len(a.st.Input)
		a.normClamp()
		if c.key == "a" || c.key == "i" || c.key == "p" || c.key == "P" {
			// stay on the last input character like a fresh normal-mode cursor
			c.n = max(1, c.n)
		}
		a.execNormal(c)
	case "u":
		a.undo()
	default:
		a.notifyErr(readOnlyMsg)
	}
}
