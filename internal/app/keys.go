package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) onKey(k *tcell.EventKey) {
	if a.st.Pending != nil {
		a.onApprovalKey(k)
		return
	}
	if a.st.Over != OverNone {
		a.onOverlayKey(k)
		return
	}
	if k.Key() == tcell.KeyCtrlC {
		a.abortTurn()
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
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.Mode = ui.ModeNormal
	case tcell.KeyEnter:
		if k.Modifiers()&tcell.ModAlt != 0 {
			a.st.Input = append(a.st.Input, '\n')
			return
		}
		a.submit()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Input); n > 0 {
			a.st.Input = a.st.Input[:n-1]
		}
	case tcell.KeyRune:
		a.st.Input = append(a.st.Input, k.Rune())
		if k.Rune() == ':' && len(a.st.Input) == 1 {
			a.st.Input = nil
			a.st.Mode = ui.ModeCommand
			a.st.Cmd = nil
		}
	}
}

func (a *App) onNormalKey(k *tcell.EventKey) {
	switch {
	case k.Rune() == 'i' || k.Rune() == 'a' || k.Key() == tcell.KeyEnter:
		a.st.Mode = ui.ModeInsert
	case k.Rune() == ':':
		a.st.Mode = ui.ModeCommand
		a.st.Cmd = nil
	case k.Rune() == 'v' || k.Rune() == 'V':
		a.st.Mode = ui.ModeVisual
		a.st.Visual = a.lastTurnIndex()
	case k.Rune() == 'j' || k.Key() == tcell.KeyDown:
		a.scrollBy(-1)
	case k.Rune() == 'k' || k.Key() == tcell.KeyUp:
		a.scrollBy(1)
	case k.Rune() == 'y':
		a.st.Yank = string(a.st.Input)
	case k.Rune() == 'p':
		a.st.Input = append(a.st.Input, []rune(a.st.Yank)...)
	case k.Rune() == 'r':
		a.runLastJS()
	case k.Rune() == '#':
		a.st.Mode = ui.ModeInsert
		a.st.Input = append(a.st.Input, '#')
	case k.Rune() == '?':
		a.openOverlay(OverHelp)
	case k.Rune() == 'q':
		a.quit = true
	case k.Key() == tcell.KeyEsc:
		a.st.Input = nil
	}
}

func (a *App) onVisualKey(k *tcell.EventKey) {
	switch {
	case k.Key() == tcell.KeyEsc || k.Rune() == 'n':
		a.st.Mode = ui.ModeNormal
	case k.Rune() == 'j' || k.Key() == tcell.KeyDown:
		a.st.Visual = a.nextJSTurn(a.st.Visual, 1)
	case k.Rune() == 'k' || k.Key() == tcell.KeyUp:
		a.st.Visual = a.nextJSTurn(a.st.Visual, -1)
	case k.Rune() == 'y':
		a.st.Yank = a.selectedCode()
		a.st.Mode = ui.ModeNormal
	case k.Rune() == 'p':
		a.st.Input = append(a.st.Input, []rune(a.selectedCode())...)
		a.st.Mode = ui.ModeInsert
	case k.Rune() == 'r':
		a.runSelectedJS()
		a.st.Mode = ui.ModeNormal
	}
}

func (a *App) onCommandKey(k *tcell.EventKey) {
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.Mode = ui.ModeNormal
		a.st.Cmd = nil
	case tcell.KeyEnter:
		cmd := string(a.st.Cmd)
		a.st.Cmd = nil
		a.st.Mode = ui.ModeNormal
		a.runCommand(cmd)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Cmd); n > 0 {
			a.st.Cmd = a.st.Cmd[:n-1]
		}
	case tcell.KeyRune:
		a.st.Cmd = append(a.st.Cmd, k.Rune())
	}
}
