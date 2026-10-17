package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) openOverlay(o Overlay) {
	a.st.Over = o
	a.st.Sel = 0
	a.st.Field = 0
	a.st.Query = nil
	a.st.Editing = false
	a.st.Buffer = nil
}

func (a *App) closeOverlay() {
	a.openOverlay(OverNone)
}

func (a *App) backOverlay() {
	switch a.st.Over {
	case OverProviderEdit:
		a.st.Over, a.st.Sel, a.st.Field = OverProviders, max(a.st.EditIdx, 0), 0
	case OverMCPEdit:
		a.st.Over, a.st.Sel, a.st.Field = OverMCP, max(a.st.EditIdx, 0), 0
	case OverTheme:
		a.cancelTheme()
		a.closeOverlay()
	default:
		a.closeOverlay()
	}
	a.st.Editing = false
	a.st.Buffer = nil
}

func (a *App) onOverlayKey(k *tcell.EventKey) {
	if k.Key() == tcell.KeyEsc && !a.st.Editing {
		a.backOverlay()
		return
	}
	switch a.st.Over {
	case OverModel:
		a.modelKey(k)
	case OverSettings:
		a.settingsKey(k)
	case OverProviders:
		a.providersKey(k)
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
	case OverTheme:
		a.themeKey(k)
	case OverHelp:
		a.helpKey(k)
	}
}

func (a *App) drawOverlay(c ui.Canvas, w, h int) {
	switch a.st.Over {
	case OverModel:
		a.drawModel(c, w, h)
	case OverSettings:
		a.drawSettings(c, w, h)
	case OverProviders:
		a.drawProviders(c, w, h)
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
	case OverTheme:
		a.drawTheme(c, w, h)
	case OverHelp:
		a.drawHelp(c, w, h)
	}
}

func clampSel(sel, n int) int {
	if n == 0 {
		return 0
	}
	return min(max(sel, 0), n-1)
}

func (a *App) navKey(k *tcell.EventKey, n int, jk bool) bool {
	up := k.Key() == tcell.KeyUp || (jk && k.Rune() == 'k')
	down := k.Key() == tcell.KeyDown || (jk && k.Rune() == 'j')
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

func (a *App) persistSettings() {
	for _, rt := range a.rtList {
		rt.VM.SetTimeout(a.st.Settings.TimeoutDuration())
	}
	if err := a.store.SaveSettings(a.st.Settings); err != nil {
		a.notifyErr(err.Error())
	}
}
