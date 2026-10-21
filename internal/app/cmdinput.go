package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// cmdKey handles a key while the command popup is active. It reports whether
// the key was consumed.
func (a *App) cmdKey(k *tcell.EventKey) bool {
	rows := commandRows(a)
	switch k.Key() {
	case tcell.KeyEsc:
		if t, ok := a.cmdToken(); ok {
			a.st.CmdDismiss = t.start + 1
		}
		return true
	case tcell.KeyUp:
		a.st.CmdSel, a.st.CmdPicked = clampSel(a.st.CmdSel-1, len(rows)), true
		return true
	case tcell.KeyDown:
		a.st.CmdSel, a.st.CmdPicked = clampSel(a.st.CmdSel+1, len(rows)), true
		return true
	case tcell.KeyTab:
		if len(rows) > 0 {
			a.cmdComplete(rows[clampSel(a.st.CmdSel, len(rows))])
		}
		return true
	case tcell.KeyEnter:
		if k.Modifiers()&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0 {
			return false // newline
		}
		if len(rows) > 0 && rows[clampSel(a.st.CmdSel, len(rows))] == "/skill:" {
			a.cmdComplete("/skill:")
			return true
		}
		a.cmdRun(rows)
		return true
	}
	a.st.CmdSel, a.st.CmdPicked = 0, false
	return false
}

// cmdComplete replaces the token with the chosen row ("/name" or "/cd path/").
func (a *App) cmdComplete(row string) {
	t, ok := a.cmdToken()
	if !ok {
		return
	}
	in := a.st.Input
	out := append(append(append([]rune(nil), in[:t.start]...), []rune(row)...), in[t.end:]...)
	a.endInsert()
	a.st.Input, a.st.Cursor, a.st.WantSet = out, t.start+len([]rune(row)), false
	a.st.InsBase = a.snapshot()
	a.st.CmdSel, a.st.CmdPicked = 0, false
}

// cmdRun cuts the token out of the draft and runs the command. A failed
// command puts the token back so it can be fixed.
func (a *App) cmdRun(rows []string) {
	t, ok := a.cmdToken()
	if !ok {
		return
	}
	cmd := t.query
	if strings.TrimSpace(cmd) == "" && !(a.st.CmdPicked && len(rows) > 0) {
		return
	}
	if len(rows) > 0 && !strings.HasPrefix(cmd, "cd ") {
		cmd = rows[clampSel(a.st.CmdSel, len(rows))][1:]
	}
	a.endInsert()
	before := a.snapshot()
	in := a.st.Input
	a.st.Input = append(append([]rune(nil), in[:t.start]...), in[t.end:]...)
	a.st.Cursor, a.st.WantSet = t.start, false
	if len(a.st.Undo) > 0 || len(before.in) > 0 {
		a.pushUndo(before)
	}
	a.st.InsBase = a.snapshot()
	a.st.CmdSel, a.st.CmdPicked = 0, false
	a.commandFailed = false
	a.runCommand(cmd)
	if a.commandFailed {
		if n := len(a.st.Undo); n > 0 {
			a.st.Undo = a.st.Undo[:n-1]
		}
		a.restore(before)
		a.st.InsBase = before
	}
}
