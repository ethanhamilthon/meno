package app

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) startVisual(kind rune) {
	t, top := a.doc()
	g := t.clamp(a.curG(top), false)
	a.setCurG(g, top)
	a.setAnchorG(g, top)
	a.st.VKind, a.st.Mode, a.st.VimBuf = kind, ui.ModeVisual, nil
}

func (a *App) exitVisual() {
	a.st.Mode, a.st.VKind, a.st.VimBuf = ui.ModeNormal, 0, nil
	a.normClampCursor()
}

// normClampCursor keeps whichever cursor is active on a character.
func (a *App) normClampCursor() {
	if a.st.InLog {
		t := a.logText()
		if len(t) == 0 {
			a.st.InLog = false
			a.normClamp()
			return
		}
		p := t.clamp(vpos{a.st.LogLine, a.st.LogCol}, false)
		a.st.LogLine, a.st.LogCol = p.line, p.col
		return
	}
	a.normClamp()
}

// selOrder returns the selection ends in document order.
func (a *App) selOrder(top int) (s, e vpos) {
	s, e = a.anchorG(top), a.curG(top)
	if e.less(s) {
		s, e = e, s
	}
	return
}

// selRngs turns the selection into flat ranges of the document: one range for
// v and V, one per line for a block.
func (a *App) selRngs(t vtext, top int) []rng {
	s, e := a.selOrder(top)
	switch a.st.VKind {
	case 'V':
		return []rng{t.span(s, e, mLine)}
	case 'b':
		an, cu := a.anchorG(top), a.curG(top)
		c0, c1 := min(an.col, cu.col), max(an.col, cu.col)+1
		var out []rng
		for l := s.line; l <= e.line && l < len(t); l++ {
			if c0 >= len(t[l]) {
				continue
			}
			out = append(out, rng{t.idx(vpos{l, c0}), t.idx(vpos{l, min(c1, len(t[l]))}), false})
		}
		return out
	}
	return []rng{t.span(s, e, mIncl)}
}

func joinRngs(t vtext, rs []rng) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = t.slice(r)
	}
	return strings.Join(parts, "\n")
}

// selSpan reports the selected rune columns [lo, hi) of document line l (n is
// its length); ok is false when the line is not selected at all.
func (a *App) selSpan(top, l, n int) (lo, hi int, ok bool) {
	if a.st.Mode != ui.ModeVisual {
		return 0, 0, false
	}
	s, e := a.selOrder(top)
	if l < s.line || l > e.line {
		return 0, 0, false
	}
	switch a.st.VKind {
	case 'V':
		return 0, n, true
	case 'b':
		an, cu := a.anchorG(top), a.curG(top)
		return min(an.col, cu.col), min(max(an.col, cu.col)+1, n), true
	}
	lo, hi = 0, n
	if l == s.line {
		lo = s.col
	}
	if l == e.line {
		hi = min(n, e.col+1)
	}
	return lo, hi, true
}

func (a *App) onVisualKey(k *tcell.EventKey) {
	r := normalRune(k)
	if r == 0 {
		switch k.Key() {
		case tcell.KeyEsc:
			a.exitVisual()
		case tcell.KeyCtrlV:
			a.switchVisual('b')
		case tcell.KeyCtrlE:
			a.scrollBy(-1)
		case tcell.KeyCtrlY:
			a.scrollBy(1)
		case tcell.KeyCtrlD, tcell.KeyCtrlF:
			a.pageCursor(a.halfPage(k.Key() == tcell.KeyCtrlF), "j")
		case tcell.KeyCtrlU, tcell.KeyCtrlB:
			a.pageCursor(a.halfPage(k.Key() == tcell.KeyCtrlB), "k")
		}
		return
	}
	if r == '\n' {
		return
	}
	if r == ':' && len(a.st.VimBuf) == 0 {
		a.exitVisual()
		a.st.Mode, a.st.Cmd = ui.ModeCommand, nil
		return
	}
	a.st.VimBuf = append(a.st.VimBuf, r)
	cmd, state := parseVim(a.st.VimBuf, true)
	switch state {
	case parseInvalid:
		a.st.VimBuf = nil
	case parseDone:
		a.st.VimBuf = nil
		a.execVisual(cmd)
	}
}

func (a *App) pageCursor(n int, key string) {
	if a.st.Mode == ui.ModeVisual || a.st.Mode == ui.ModeNormal {
		a.moveVertical(vcmd{n: n, key: key})
	}
}

func (a *App) switchVisual(kind rune) {
	if a.st.VKind == kind {
		a.exitVisual()
		return
	}
	a.st.VKind = kind
}

func (a *App) execVisual(c vcmd) {
	t, top := a.doc()
	cur := t.clamp(a.curG(top), false)
	switch c.key {
	case "v", "V":
		a.switchVisual(rune(c.key[0]))
	case "j", "k":
		a.moveVertical(c)
	case "gg", "G":
		lo, hi := top, len(t)-1
		if a.st.InLog {
			lo, hi = 0, top-1
		}
		line := lo
		if c.key == "G" {
			line = hi
		}
		if c.hasN {
			line = min(lo+c.n-1, hi)
		}
		a.setCurG(vpos{line, firstNonBlank(t[line])}, top)
		a.followCursor()
	case "h", "l", "0", "^", "$", "w", "W", "b", "B", "e", "E", "{", "}", "f", "F", "t", "T", ";", ",":
		if to, _, ok := a.resolveMotion(t, cur, c, false); ok {
			a.setCurG(t.clamp(to, false), top)
			a.st.WantSet = false
			a.followCursor()
		}
	case "o", "O":
		an := a.anchorG(top)
		a.setAnchorG(cur, top)
		a.setCurG(an, top)
		a.followCursor()
	case "iw", "aw", "iW", "aW", "i\"", "a\"", "i'", "a'", "i`", "a`", "i(", "a(", "ib", "i)", "a)", "i[", "a[", "i]", "a]", "i{", "a{", "iB", "i}", "a}", "i<", "a<", "i>", "a>", "ip", "ap":
		if r, ok := t.object(cur, c.key); ok && r.hi > r.lo {
			a.setAnchorG(t.pos(r.lo), top)
			a.setCurG(t.pos(r.hi-1), top)
			if r.line {
				a.st.VKind = 'V'
			}
			a.followCursor()
		}
	case "y", "Y":
		a.visualYank(t, top, c.key == "Y")
	case "d", "x", "X", "D", "c", "s", "C", "S", "p", "P", "~", "u", "U", "J", "r":
		a.visualEdit(t, top, c)
	case "gu", "gU", "g~", "gJ":
		a.visualEdit(t, top, vcmd{key: map[string]string{"gu": "u", "gU": "U", "g~": "~", "gJ": "J"}[c.key]})
	}
}

func (a *App) visualYank(t vtext, top int, lines bool) {
	if lines {
		a.st.VKind = 'V'
	}
	rs := a.selRngs(t, top)
	s, _ := a.selOrder(top)
	a.setReg(joinRngs(t, rs), a.st.VKind == 'V')
	a.exitVisual()
	a.setCurG(t.clamp(s, false), top)
	a.followCursor()
}

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
	a.st.Mode, a.st.VKind, a.st.VimBuf = ui.ModeNormal, 0, nil
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
