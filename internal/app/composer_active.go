package app

import "meno/internal/ui"

func (a *App) activeComposerHeight(w int) int {
	text := a.st.Input
	if a.st.Mode == ui.ModeCommand {
		text = a.st.Cmd
	}
	return 1 + min(composerMaxText, max(1, len(layoutInput(text, a.composerInnerWidth(w))))) + composerPadY
}

// withCommandDraft reuses the message editor's geometry without sharing drafts.
func (a *App) withCommandDraft(fn func()) {
	input, cursor, top := a.st.Input, a.st.Cursor, a.st.InputTop
	a.st.Input, a.st.Cursor, a.st.InputTop = a.st.Cmd, a.st.CmdCursor, a.st.CmdTop
	fn()
	a.st.Cmd, a.st.CmdCursor, a.st.CmdTop = a.st.Input, a.st.Cursor, a.st.InputTop
	a.st.Input, a.st.Cursor, a.st.InputTop = input, cursor, top
}

func (a *App) drawActiveComposer(c ui.Canvas, x, y, w, h int) {
	if a.st.Mode != ui.ModeCommand {
		a.drawComposer(c, x, y, w, h)
		return
	}
	a.withCommandDraft(func() { a.drawComposer(c, x, y, w, h) })
	c.Text(x+max(0, composerPadX-1), y+1, 1, ":", ui.Style{FG: ui.Yellow, BG: ui.BgTerm}.TCell())
}

func (a *App) activeCompletionHeight(w int) int {
	if a.st.Mode == ui.ModeCommand {
		return acRowCount(len(commandRows(a)), w)
	}
	return a.acHeight()
}
