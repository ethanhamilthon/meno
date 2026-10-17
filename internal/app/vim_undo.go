package app

import (
	"slices"

	"meno/internal/ui"
)

// snap is an undo point of the input.
type snap struct {
	in  []rune
	cur int
}

const maxUndo = 200

func (a *App) snapshot() snap {
	return snap{append([]rune(nil), a.st.Input...), a.st.Cursor}
}

func (a *App) restore(s snap) {
	a.st.Input, a.st.Cursor, a.st.WantSet = append([]rune(nil), s.in...), s.cur, false
}

// pushUndo records s as an undo point and drops the redo history.
func (a *App) pushUndo(s snap) {
	a.st.Undo = append(a.st.Undo, s)
	if len(a.st.Undo) > maxUndo {
		a.st.Undo = a.st.Undo[len(a.st.Undo)-maxUndo:]
	}
	a.st.Redo = nil
}

func (a *App) undo() {
	n := len(a.st.Undo)
	if n == 0 {
		a.notify("already at oldest change")
		return
	}
	a.st.Redo = append(a.st.Redo, a.snapshot())
	a.restore(a.st.Undo[n-1])
	a.st.Undo = a.st.Undo[:n-1]
	a.normClamp()
}

func (a *App) redo() {
	n := len(a.st.Redo)
	if n == 0 {
		a.notify("already at newest change")
		return
	}
	a.st.Undo = append(a.st.Undo, a.snapshot())
	a.restore(a.st.Redo[n-1])
	a.st.Redo = a.st.Redo[:n-1]
	a.normClamp()
}

// beginInsert enters insert mode. base is the input before the command that
// led here (so "cw", "o"... undo as one step together with the typing).
func (a *App) beginInsert(base snap) {
	if a.st.InLog {
		a.st.InLog, a.st.Cursor = false, len(a.st.Input)
	}
	a.st.Mode = ui.ModeInsert
	a.st.InsBase = base
	a.st.VimBuf = nil
}

// endInsert closes an insert session, recording one undo step if text changed.
func (a *App) endInsert() {
	if !slices.Equal(a.st.InsBase.in, a.st.Input) {
		a.pushUndo(a.st.InsBase)
	}
}

// leaveInsert is Esc in insert mode: like vim, the cursor steps back one.
func (a *App) leaveInsert() {
	a.endInsert()
	a.st.Mode = ui.ModeNormal
	a.st.VimBuf = nil
	a.clampCursor()
	t := a.inputText()
	p := t.pos(a.st.Cursor)
	if p.col > 0 {
		p.col--
	}
	a.setCursorPos(t, p)
}

// --- input as a vim document ---

func (a *App) inputText() vtext { return splitRunes(a.st.Input) }

func (a *App) setCursorPos(t vtext, p vpos) {
	a.st.Cursor = t.idx(t.clamp(p, true))
	a.st.WantSet = false
}

// setText replaces the input with t and puts the cursor at p.
func (a *App) setText(t vtext, p vpos) {
	a.st.Input = t.flat()
	a.st.Sel = 0
	a.setCursorPos(t, p)
}

// normClamp keeps the cursor on a character, as vim does in normal mode.
func (a *App) normClamp() {
	t := a.inputText()
	a.setCursorPos(t, t.clamp(t.pos(min(max(0, a.st.Cursor), len(a.st.Input))), false))
}
