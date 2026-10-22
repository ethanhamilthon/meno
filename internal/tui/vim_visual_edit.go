package tui

import (
	"unicode"

	"meno/internal/render"
)

// visualEdit applies an editing command to the selection. Only text in the
// input can change.
func (a *App) visualEdit(t vtext, top int, c vcmd) {
	rs := a.selRngs(t, top)
	base := t.idx(vpos{top, 0})
	if len(rs) == 0 {
		a.exitVisual()
		return
	}
	for _, r := range rs {
		if r.lo < base {
			a.notifyErr(readOnlyMsg)
			return
		}
	}
	linewise := a.st.VKind == 'V'
	switch c.key {
	case "D", "X", "C", "S":
		linewise = true
		if a.st.VKind == 'b' && (c.key == "D" || c.key == "C") {
			linewise = false
		}
		if linewise {
			s, e := a.selOrder(top)
			rs = []rng{t.span(s, e, mLine)}
		}
	}
	// ranges relative to the input
	in := a.inputText()
	rel := make([]rng, len(rs))
	for i, r := range rs {
		rel[i] = rng{r.lo - base, r.hi - base, linewise || (r.line && a.st.VKind == 'V')}
	}
	snapBefore := a.snapshot()
	s, _ := a.selOrder(top)
	startPos := vpos{s.line - top, s.col}
	text := joinRngs(in, rel)
	a.st.Mode, a.st.VKind, a.st.VimBuf = render.ModeInput, 0, nil
	switch c.key {
	case "d", "x", "X", "D":
		a.setReg(text, linewise)
		a.pushUndo(snapBefore)
		nt, np := cutAll(in, rel)
		a.setText(nt, nt.clamp(np, false))
	case "c", "s", "C", "S":
		a.setReg(text, linewise)
		if linewise {
			lo, hi := in.pos(rel[0].lo), in.pos(rel[0].hi)
			nt := append(append(append(vtext(nil), in[:lo.line]...), nil), in[hi.line+1:]...)
			a.setText(nt, vpos{lo.line, 0})
		} else {
			nt, np := cutAll(in, rel)
			a.setText(nt, np)
		}
		a.beginInsert(snapBefore)
	case "p", "P":
		old, oldLine := a.st.Reg, a.st.RegLine
		if old == "" {
			return
		}
		a.setReg(text, linewise)
		a.pushUndo(snapBefore)
		nt, np := cutAll(in, rel)
		if linewise {
			nt2, np2 := nt.put(vpos{min(np.line, len(nt)-1), 0}, old, oldLine, true)
			if oldLine && np.line >= len(nt) {
				nt2, np2 = nt.put(vpos{len(nt) - 1, 0}, old, oldLine, false)
			}
			if !oldLine { // charwise text replacing whole lines becomes a line of its own
				nt2, np2 = nt.put(vpos{min(np.line, len(nt)-1), 0}, old+"\n", true, true)
			}
			a.setText(nt2, nt2.clamp(np2, false))
			return
		}
		before := np.col < len(nt[np.line]) || len(nt[np.line]) == 0
		if !before {
			np.col = len(nt[np.line]) - 1
		}
		nt2, np2 := nt.put(np, old, oldLine, before)
		a.setText(nt2, nt2.clamp(np2, false))
	case "~", "u", "U":
		a.pushUndo(snapBefore)
		f := in.flat()
		for _, r := range rel {
			for i := r.lo; i < r.hi && i < len(f); i++ {
				switch c.key {
				case "~":
					f[i] = toggleRune(f[i])
				case "u":
					f[i] = unicode.ToLower(f[i])
				default:
					f[i] = unicode.ToUpper(f[i])
				}
			}
		}
		a.setText(splitRunes(f), startPos)
	case "r":
		if c.ch == 0 {
			return
		}
		a.pushUndo(snapBefore)
		f := in.flat()
		for _, r := range rel {
			for i := r.lo; i < r.hi && i < len(f); i++ {
				if f[i] != '\n' {
					f[i] = c.ch
				}
			}
		}
		a.setText(splitRunes(f), startPos)
	case "J":
		a.pushUndo(snapBefore)
		sl, el := in.pos(rel[0].lo).line, in.pos(rel[len(rel)-1].hi).line
		nt, np := in.joinLines(sl, max(2, el-sl+1))
		a.setText(nt, np)
	}
	a.followCursor()
}

// cutAll deletes ranges (given in document order) from the last to the first.
func cutAll(in vtext, rs []rng) (vtext, vpos) {
	p := vpos{}
	for i := len(rs) - 1; i >= 0; i-- {
		in, p = in.cut(rs[i])
	}
	return in, p
}
