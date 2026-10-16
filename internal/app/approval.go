package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type approval = approvalReq

func (a *App) onApprovalKey(k *tcell.EventKey) {
	p, ok := a.st.Pending.(approvalReq)
	if !ok {
		a.st.Pending = nil
		return
	}
	switch {
	case k.Rune() == 'y' || k.Key() == tcell.KeyEnter:
		p.reply <- true
		a.st.Pending = nil
	case k.Rune() == 'n' || k.Key() == tcell.KeyEsc:
		p.reply <- false
		a.st.Pending = nil
	}
}

func (a *App) drawApproval(c ui.Canvas, w, h int) {
	p, ok := a.st.Pending.(approvalReq)
	if !ok {
		return
	}
	msg := " allow " + p.action + " " + ui.Truncate(p.target, w-30) + "?  y/n "
	c.Fill(0, h-1, w, 1, ui.BgJSRun)
	c.Text(0, h-1, w, msg, ui.Style{FG: ui.FgWhite, BG: ui.BgJSRun}.TCell())
}
