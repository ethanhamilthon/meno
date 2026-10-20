package app

import (
	"github.com/gdamore/tcell/v2"
	"slices"
)

type commandPoint struct {
	text   []rune
	cursor int
}

func (a *App) commandPoint() commandPoint {
	return commandPoint{append([]rune(nil), a.st.Cmd...), a.st.CmdCursor}
}
func (a *App) commandEdit(k *tcell.EventKey) {
	if k.Key() == tcell.KeyCtrlZ {
		a.commandUndo(false)
		return
	}
	if k.Key() == tcell.KeyCtrlR {
		a.commandUndo(true)
		return
	}
	before := a.commandPoint()
	edit := textEdit{Text: append([]rune(nil), a.st.Cmd...), Cursor: a.st.CmdCursor, Multi: true}
	edit.Handle(k)
	a.st.Cmd, a.st.CmdCursor = edit.Text, edit.Cursor
	if !slices.Equal(before.text, edit.Text) {
		a.st.CmdUndo = append(a.st.CmdUndo, before)
		if len(a.st.CmdUndo) > maxUndo {
			a.st.CmdUndo = a.st.CmdUndo[1:]
		}
		a.st.CmdRedo = nil
	}
	a.st.CmdSel = 0
}
func (a *App) commandUndo(redo bool) {
	from, to := &a.st.CmdUndo, &a.st.CmdRedo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	*to = append(*to, a.commandPoint())
	point := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	a.st.Cmd, a.st.CmdCursor = point.text, point.cursor
}
