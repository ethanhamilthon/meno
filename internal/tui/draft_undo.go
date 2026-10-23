package tui

import (
	"slices"
)

// snap is an undo point of the input.
type snap struct {
	in   []rune
	cur  int
	imgs []draftImage // the images the tokens refer to, so undo restores them too
}

const maxUndo = 200

func (a *App) snapshot() snap {
	return a.st.Runtime.snapshot()
}

func (a *App) restore(s snap) {
	a.st.Input, a.st.Cursor, a.st.WantSet = append([]rune(nil), s.in...), s.cur, false
	if s.imgs != nil {
		a.st.Images = append([]draftImage(nil), s.imgs...)
	}
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
	a.clampCursor()
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
	a.clampCursor()
}

// endInsert closes an editing session, recording one undo step if text changed.
func (a *App) endInsert() {
	if !slices.Equal(a.st.InsBase.in, a.st.Input) {
		a.pushUndo(a.st.InsBase)
	}
}

// --- input as a document ---

func (a *App) inputText() vtext { return splitRunes(a.st.Input) }
