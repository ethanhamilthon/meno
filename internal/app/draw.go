package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) draw() {
	w, h := a.scr.Size()
	if a.st.InLog && (a.timelineEmpty() || len(a.logText()) == 0) {
		a.st.InLog = false // nothing to browse: the cursor lives in the input
	}
	a.scr.Fill(' ', tcell.StyleDefault.Background(ui.BgTerm))
	c := ui.Canvas{S: a.scr}
	a.cur = cursorPos{}
	a.waveOn.Store(false)
	a.drawMain(c, w, h)
	if a.st.Over != OverNone {
		a.drawOverlay(c, w, h)
	} else if a.st.Ask != nil {
		a.drawAskForm(c, w, h)
	}
	a.applyCursor()
	a.scr.Show()
	a.placeImages()
}
