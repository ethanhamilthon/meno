package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

func (a *App) startVisual(kind rune) {
	t, top := a.doc()
	g := t.clamp(a.curG(top), false)
	a.setCurG(g, top)
	a.setAnchorG(g, top)
	a.st.VKind, a.st.Mode, a.st.VimBuf = kind, render.ModeVisual, nil
}

func (a *App) exitVisual() {
	// Keep the clipboard selection after leaving VISUAL.
	t, top := a.doc()
	a.st.SelText = joinRngs(t, a.selRngs(t, top))
	a.st.Mode, a.st.VKind, a.st.VimBuf = render.ModeInput, 0, nil
	a.st.InLog = false
	a.clampCursor()
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
	if a.st.Mode != render.ModeVisual {
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
	if k.Key() == tcell.KeyEnter && a.st.AnchorLog {
		t, top := a.doc()
		_ = t
		a.commentSelection(top)
		return
	}
	r := a.vimRune(k, true)
	if r == 0 {
		switch k.Key() {
		case tcell.KeyEsc:
			inLog := a.st.InLog
			a.exitVisual()
			a.st.InLog = inLog && len(a.logText()) > 0
			a.st.Mode = render.ModeEdit
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
	if r == ':' || r == '\n' {
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
	if a.st.Mode == render.ModeVisual || a.st.Mode == render.ModeInput {
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
	case "c":
		if a.st.AnchorLog || a.st.InLog {
			// A selection in the conversation: "c" quotes it. In the draft it
			// still changes the text, as before.
			a.commentSelection(top)
			return
		}
		a.visualEdit(t, top, c)
	case "d", "x", "X", "D", "s", "C", "S", "p", "P", "~", "u", "U", "J", "r":
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
	if s.line >= top {
		a.setCurG(t.clamp(s, false), top)
	}
	a.followCursor()
}
