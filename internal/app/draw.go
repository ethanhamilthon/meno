package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) draw() {
	w, h := a.scr.Size()
	a.scr.Fill(' ', tcell.StyleDefault.Background(ui.BgTerm))
	c := ui.Canvas{S: a.scr}
	a.drawMain(c, w, h)
	if a.st.Over != OverNone {
		a.drawOverlay(c, w, h)
	}
	if a.st.Pending != nil {
		a.drawApproval(c, w, h)
	}
	a.scr.Show()
}
