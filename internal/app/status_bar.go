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
	textColors := a.chromeColors(w, chromeText())
	ruleColors := a.chromeColors(w, ui.FgFaint)
	a.chromeText(c, x, y, w, strings.Repeat("─", w), ui.FgFaint, ruleColors)
	gutter := min(ui.Gutter, max(0, (w-1)/2))
	lo, hi := x+gutter, x+w-gutter
	available := max(0, hi-lo)
	right := ui.Truncate(a.ctxLabel(), available)
	rw := ui.TextWidth(right)
	rx := hi - rw
	left := ui.Truncate(a.footerModel(), max(0, available-rw-1))
	lw := ui.TextWidth(left)
	mid := joinSegText(a.globalCounts())
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
	a.chromeText(c, lo, y, lw, left, chromeText(), textColors)
	a.chromeText(c, rx, y, rw, right, chromeText(), textColors)
	if mid != "" {
		a.chromeText(c, mx, y, mw, mid, chromeText(), textColors)
	}
}
