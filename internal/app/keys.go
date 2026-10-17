package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) onKey(k *tcell.EventKey) {
	if k.Key() == tcell.KeyCtrlC {
		a.abortTurn()
		return
	}
	if a.st.Over != OverNone {
		a.onOverlayKey(k)
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
		if a.st.Mode != ui.ModeInsert {
			a.st.Scroll = 0
			if a.st.Mode == ui.ModeNormal {
				a.st.InLog = false // follow the tail: back to the input
			}
			return
		}
	}
	if a.st.Ask != nil && a.st.Mode != ui.ModeCommand && a.askKey(k) {
		return
	}
	switch a.st.Mode {
	case ui.ModeInsert:
		a.onInsertKey(k)
	case ui.ModeNormal:
		a.onNormalKey(k)
	case ui.ModeVisual:
		a.onVisualKey(k)
	case ui.ModeCommand:
		a.onCommandKey(k)
	}
}

func (a *App) onInsertKey(k *tcell.EventKey) {
	a.clampCursor()
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
		a.moveVert(-1)
	case tcell.KeyDown:
		a.moveVert(1)
	case tcell.KeyHome, tcell.KeyCtrlA:
		a.moveLineEdge(-1)
	case tcell.KeyEnd, tcell.KeyCtrlE:
		a.moveLineEdge(1)
	case tcell.KeyRune:
		// Commands are not available while typing; leave insert mode first (esc, then ':').
		a.insertAtCursor([]rune{k.Rune()})
	}
}

func (a *App) onNormalKey(k *tcell.EventKey) {
	r := normalRune(k)
	if r == 0 {
		switch k.Key() {
		case tcell.KeyEsc:
			a.st.VimBuf = nil
		case tcell.KeyCtrlR:
			a.redo()
		case tcell.KeyCtrlV:
			a.startVisual('b')
		case tcell.KeyCtrlE:
			a.scrollBy(-1)
		case tcell.KeyCtrlY:
			a.scrollBy(1)
		case tcell.KeyCtrlD, tcell.KeyCtrlF:
			a.scrollBy(-a.halfPage(k.Key() == tcell.KeyCtrlF))
		case tcell.KeyCtrlU, tcell.KeyCtrlB:
			a.scrollBy(a.halfPage(k.Key() == tcell.KeyCtrlB))
		}
		return
	}
	// Keys that are not part of a pending command.
	if len(a.st.VimBuf) == 0 {
		switch r {
		case '\n':
			a.beginInsert(a.snapshot())
			return
		case ':':
			a.st.Mode = ui.ModeCommand
			a.st.Cmd = nil
			return
		case 'v', 'V':
			a.startVisual(r)
			return
		case '#':
			a.beginInsert(a.snapshot())
			a.insertAtCursor([]rune{'#'})
			return
		case '?':
			a.openOverlay(OverHelp)
			return
		case 'q':
			a.runCommand("q")
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
	}
}

// normalRune maps a key to the vim key it stands for (0 = not a text key).
func normalRune(k *tcell.EventKey) rune {
	switch k.Key() {
	case tcell.KeyRune:
		if k.Rune() == ' ' {
			return 'l'
		}
		return k.Rune()
	case tcell.KeyLeft, tcell.KeyBackspace, tcell.KeyBackspace2:
		return 'h'
	case tcell.KeyRight:
		return 'l'
	case tcell.KeyUp:
		return 'k'
	case tcell.KeyDown:
		return 'j'
	case tcell.KeyHome:
		return '0'
	case tcell.KeyDelete:
		return 'x'
	case tcell.KeyEnter:
		return '\n'
	}
	return 0
}

func (a *App) halfPage(full bool) int {
	_, h := a.scr.Size()
	if full {
		return max(1, h-a.bottomHeight()-1)
	}
	return max(1, (h-a.bottomHeight())/2)
}

func (a *App) onCommandKey(k *tcell.EventKey) {
	rows := commandRows(a)
	switch k.Key() {
	case tcell.KeyUp:
		a.st.CmdSel = clampSel(a.st.CmdSel-1, len(rows))
	case tcell.KeyDown:
		a.st.CmdSel = clampSel(a.st.CmdSel+1, len(rows))
	case tcell.KeyTab:
		if len(rows) > 0 {
			a.st.Cmd = []rune(rows[clampSel(a.st.CmdSel, len(rows))][1:])
			a.st.CmdSel = 0
		}
	case tcell.KeyEsc:
		a.st.Mode = ui.ModeNormal
		a.st.Cmd = nil
	case tcell.KeyEnter:
		cmd := string(a.st.Cmd)
		if len(rows) > 0 {
			cmd = rows[clampSel(a.st.CmdSel, len(rows))][1:]
		}
		a.st.CmdSel = 0
		a.st.Cmd = nil
		a.st.Mode = ui.ModeNormal
		a.runCommand(cmd)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Cmd); n > 0 {
			a.st.Cmd = a.st.Cmd[:n-1]
			a.st.CmdSel = 0
		}
	case tcell.KeyRune:
		a.st.Cmd = append(a.st.Cmd, k.Rune())
		a.st.CmdSel = 0
	}
}
