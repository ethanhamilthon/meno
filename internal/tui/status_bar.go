package tui

import (
	"meno/internal/render"
)

func (a *App) ctxLabel() string {
	max := "?"
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		max = compactTokens(info.Context)
	}
	return compactTokens(a.st.Context) + "/" + max
}

func (a *App) footerModel() string {
	_, model, effort := a.selection()
	if model == "" {
		model = "no model"
	}
	if effort != "" {
		model += " | " + effort
	}
	return model
}

func (a *App) drawStatus(c render.Canvas, x, y, w int) {
	if w <= 0 {
		return
	}
	gutter := min(render.Gutter, max(0, (w-1)/2))
	lo, hi := x+gutter, x+w-gutter
	available := max(0, hi-lo)
	context := render.Truncate(a.ctxLabel(), available)
	model := render.Truncate(a.footerModel(), max(0, available-render.TextWidth(context)-3))
	right := context
	if model != "" {
		right = model + " | " + context
	}
	rw := render.TextWidth(right)
	rx := hi - rw
	left := render.Truncate(a.composerHeaderLabel(), min(w*composerHeaderMax/100, max(0, available-rw-2)))
	lw := render.TextWidth(left)
	mid := a.statusMid()
	if barSegsWidth(mid) > max(0, available-lw-rw-4) {
		mid = compactSegs(mid)
	}
	mw := barSegsWidth(mid)
	mx := x + (w-mw)/2
	if mx < lo+lw+2 || mx+mw > rx-2 {
		mid = nil
	}
	style := render.Style{FG: chromeText(), BG: render.BgTerm}.TCell()
	c.Text(lo, y, lw, left, style)
	c.Text(rx, y, rw, right, style)
	a.win.statusMidX, a.win.statusMidSegs = mx, mid
	drawBarSegs(c, mx, y, mw, mid)
}
