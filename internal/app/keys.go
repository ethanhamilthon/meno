package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) onKey(k *tcell.EventKey) {
	k = normalizeCtrl(k)
	if k.Key() == tcell.KeyCtrlC {
		a.abortTurn()
		return
	}
	if a.st.Over != OverNone {
		a.onOverlayKey(k)
		return
	}
	if a.st.Ask != nil {
		a.askKey(k)
		return
	}
	switch k.Key() {
	case tcell.KeyPgUp:
		_, h := a.scr.Size()
		a.scrollBy(max(1, h-a.bottomHeight()-1))
		return
	case tcell.KeyPgDn:
		_, h := a.scr.Size()
		a.scrollBy(-max(1, h-a.bottomHeight()-1))
		return
	case tcell.KeyEnd:
		if a.st.Mode != ui.ModeInput {
			a.st.Scroll = 0
			if a.st.Mode == ui.ModeInput {
				a.st.InLog = false // follow the tail: back to the input
			}
			return
		}
	}
	switch a.st.Mode {
	case ui.ModeInput:
		a.onInsertKey(k)
	case ui.ModeVisual:
		a.onVisualKey(k)
	case ui.ModeCommand:
		a.onCommandKey(k)
	}
}

func (a *App) onInsertKey(k *tcell.EventKey) {
	a.clampCursor()
	defer a.pickSync()
	if a.acActive() && a.acKey(k) {
		return
	}
	word := k.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) != 0
	switch k.Key() {
	case tcell.KeyEsc:
		a.leaveInsert()
	case tcell.KeyEnter:
		// Shift+Enter needs a terminal that reports it (kitty/CSI-u protocol);
		// Alt+Enter and Ctrl+J work everywhere.
		if k.Modifiers()&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0 {
			a.insertNewline()
			return
		}
		a.submit()
	case tcell.KeyCtrlJ:
		a.insertNewline()
	case tcell.KeyCtrlZ:
		a.endInsert()
		a.undo()
	case tcell.KeyCtrlR:
		a.redo()
	case tcell.KeyCtrlV:
		a.attachFromClipboard(false)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.backspace()
	case tcell.KeyDelete:
		a.deleteForward()
	case tcell.KeyLeft:
		if word {
			a.moveWord(-1)
		} else {
			a.moveLeft()
		}
	case tcell.KeyRight:
		if word {
			a.moveWord(1)
		} else {
			a.moveRight()
		}
	case tcell.KeyUp:
		if len(a.st.Input) == 0 && a.recallQueue() {
			return
		}
		a.moveVert(-1)
	case tcell.KeyDown:
		a.moveVert(1)
	case tcell.KeyHome, tcell.KeyCtrlA:
		a.moveLineEdge(-1)
	case tcell.KeyEnd, tcell.KeyCtrlE:
		a.moveLineEdge(1)
	case tcell.KeyRune:
		// INPUT never intercepts colon or navigation letters.
		if !isObjRune(k.Rune()) {
			a.insertAtCursor([]rune{k.Rune()})
		}
	}
}

func (a *App) onCommandKey(k *tcell.EventKey) {
	rows := commandRows(a)
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.Mode = ui.ModeInput
	case tcell.KeyUp:
		a.st.CmdSel = clampSel(a.st.CmdSel-1, len(rows))
	case tcell.KeyDown:
		a.st.CmdSel = clampSel(a.st.CmdSel+1, len(rows))
	case tcell.KeyTab:
		if len(rows) > 0 {
			a.st.Cmd = []rune(rows[clampSel(a.st.CmdSel, len(rows))][1:])
			a.st.CmdCursor = len(a.st.Cmd)
			a.st.CmdSel = 0
		}
	case tcell.KeyEnter:
		cmd := string(a.st.Cmd)
		if strings.TrimSpace(cmd) == "" {
			return
		}
		if len(rows) > 0 && !strings.HasPrefix(cmd, "cd ") {
			cmd = rows[clampSel(a.st.CmdSel, len(rows))][1:]
		}
		a.commandFailed = false
		a.runCommand(cmd)
		if !a.commandFailed {
			a.st.Cmd = nil
			a.st.CmdCursor = 0
			a.st.CmdUndo = nil
			a.st.CmdRedo = nil
		}
	default:
		a.commandEdit(k)
	}
}
