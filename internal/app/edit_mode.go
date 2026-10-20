package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// startEdit enters EDIT: a vim-normal-like mode on the draft, or on the end of
// the timeline when the draft is empty. Nothing is selected until v is pressed.
func (a *App) startEdit() {
	a.st.VimBuf, a.st.VKind = nil, 0
	a.st.Mode = ui.ModeEdit
	a.st.InLog = false
	if len(a.st.Input) == 0 && len(a.logText()) > 0 {
		a.st.InLog = true
		a.execLogNormal(vcmd{key: "G", n: 1})
	}
	a.normClamp()
}

// onEditKey handles a key in EDIT. Esc returns to INPUT, v / V
// / Ctrl+V start a selection, and the rest are vim normal commands.
func (a *App) onEditKey(k *tcell.EventKey) {
	r := a.vimRune(k, false)
	if r == 0 {
		switch k.Key() {
		case tcell.KeyEsc:
			a.st.Mode, a.st.VimBuf = ui.ModeInput, nil
			a.st.InLog = false
		case tcell.KeyCtrlV:
			a.startVisual('b')
		case tcell.KeyCtrlR:
			a.redo()
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
	if len(a.st.VimBuf) == 0 {
		switch r {
		case ':':
			return
		case 'v', 'V':
			a.startVisual(r)
			return
		case '\n':
			return
		}
	}
	a.st.VimBuf = append(a.st.VimBuf, r)
	cmd, state := parseVim(a.st.VimBuf, false)
	switch state {
	case parseInvalid:
		a.st.VimBuf = nil
	case parseDone:
		a.st.VimBuf = nil
		a.execNormal(cmd)
		a.postEdit()
	}
}

// postEdit keeps the cursor on a character after a command that stays in EDIT.
func (a *App) postEdit() {
	if a.st.Mode == ui.ModeEdit && !a.st.InLog {
		a.normClamp()
	}
}
