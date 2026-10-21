package app

import (
	"meno/internal/ui"
	"strings"
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

func (a *App) drawStatus(c ui.Canvas, x, y, w int) {
	if w <= 0 {
		return
	}
	gutter := min(ui.Gutter, max(0, (w-1)/2))
	lo, hi := x+gutter, x+w-gutter
	available := max(0, hi-lo)
	context := ui.Truncate(a.ctxLabel(), available)
	model := ui.Truncate(a.footerModel(), max(0, available-ui.TextWidth(context)-3))
	right := context
	if model != "" {
		right = model + " | " + context
	}
	rw := ui.TextWidth(right)
	rx := hi - rw
	left := ui.Truncate(a.composerHeaderLabel(), min(w*composerHeaderMax/100, max(0, available-rw-2)))
	lw := ui.TextWidth(left)
	mid := joinSegText(a.globalCounts())
	if c := a.connLabel(); c != "" {
		mid = c
	}
	if ui.TextWidth(mid) > max(0, available-lw-rw-4) {
		parts := strings.Split(mid, " | ")
		for i, p := range parts {
			fields := strings.Fields(p)
			if len(fields) == 2 {
				parts[i] = fields[0] + strings.ToLower(fields[1][:1])
			}
		}
		mid = strings.Join(parts, " | ")
	}
	mw := ui.TextWidth(mid)
	mx := x + (w-mw)/2
	if mx < lo+lw+2 || mx+mw > rx-2 {
		mid = ""
	}
	style := ui.Style{FG: chromeText(), BG: ui.BgTerm}.TCell()
	c.Text(lo, y, lw, left, style)
	c.Text(rx, y, rw, right, style)
	if mid != "" {
		c.Text(mx, y, mw, mid, style)
	}
}
