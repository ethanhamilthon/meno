package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) openOverlay(o Overlay) {
	if a.st.Over == OverNone && o != OverNone {
		a.st.OverlayOrigin = a.st.Mode
	}
	a.st.Over = o
	a.st.Sel = 0
	a.st.Field = 0
	a.st.Query.Reset()
	a.endEdit()
	a.st.FormNote = ""
}

func (a *App) closeOverlay() {
	a.st.Mode = a.st.OverlayOrigin
	a.openOverlay(OverNone)
}

func (a *App) backOverlay() {
	switch a.st.Over {
	case OverProviderEdit:
		a.st.Over, a.st.Sel, a.st.Field = OverLogin, 0, 0
		if a.st.EditIdx >= 0 {
			a.st.Sel = a.loginRowOfProvider(a.st.EditIdx)
		}
		a.afterProvidersChanged()
	case OverLoginFlow:
		a.cancelFlow()
		a.st.Over, a.st.Sel = OverLogin, 0
	case OverMCPEdit:
		a.st.Over, a.st.Sel, a.st.Field = OverMCP, max(a.st.EditIdx, 0), 0
	case OverTasks:
		if a.st.TasksUI.LogID != "" {
			a.st.TasksUI.LogID = ""
		} else {
			a.closeOverlay()
		}
	default:
		a.closeOverlay()
	}
	a.endEdit()
	a.st.FormNote = ""
}

func (a *App) onOverlayKey(k *tcell.EventKey) {
	if k.Key() == tcell.KeyEsc && !a.st.Editing {
		if a.st.Locked && a.st.Over == OverLogin {
			a.notify("add a provider to start · ctrl+c quits")
			return
		}
		a.backOverlay()
		return
	}
	switch a.st.Over {
	case OverModel:
		a.modelKey(k)
	case OverSettings:
		a.settingsKey(k)
	case OverLogin:
		a.loginKey(k)
	case OverLoginFlow:
		a.loginFlowKey(k)
	case OverProviderEdit:
		a.providerEditKey(k)
	case OverSkills:
		a.skillsKey(k)
	case OverMCP:
		a.mcpKey(k)
	case OverMCPEdit:
		a.mcpEditKey(k)
	case OverSessions:
		a.sessionsKey(k)
	case OverContext:
		a.onContextKey(k)
	case OverHelp:
		a.helpKey(k)
	case OverFork:
		a.forkKey(k)
	case OverTasks:
		a.tasksKey(k)
	}
}

func (a *App) drawOverlay(c ui.Canvas, w, h int) {
	switch a.st.Over {
	case OverModel:
		a.drawModel(c, w, h)
	case OverSettings:
		a.drawSettings(c, w, h)
	case OverLogin:
		a.drawLogin(c, w, h)
	case OverLoginFlow:
		a.drawLoginFlow(c, w, h)
	case OverProviderEdit:
		a.drawProviderEdit(c, w, h)
	case OverSkills:
		a.drawSkills(c, w, h)
	case OverMCP:
		a.drawMCP(c, w, h)
	case OverMCPEdit:
		a.drawMCPEdit(c, w, h)
	case OverSessions:
		a.drawSessions(c, w, h)
	case OverContext:
		a.drawContext(c, w, h)
	case OverHelp:
		a.drawHelp(c, w, h)
	case OverFork:
		a.drawFork(c, w, h)
	case OverTasks:
		a.drawTasks(c, w, h)
	}
}

func clampSel(sel, n int) int {
	if n == 0 {
		return 0
	}
	return min(max(sel, 0), n-1)
}

func (a *App) navKey(k *tcell.EventKey, n int, jk bool) bool {
	up := k.Key() == tcell.KeyUp || (jk && isKey(k, 'k'))
	down := k.Key() == tcell.KeyDown || (jk && isKey(k, 'j'))
	switch {
	case down:
		a.st.Sel = clampSel(a.st.Sel+1, n)
	case up:
		a.st.Sel = clampSel(a.st.Sel-1, n)
	default:
		return false
	}
	return true
}
