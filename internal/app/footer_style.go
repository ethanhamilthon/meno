package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"meno/internal/ui"
)

func chromeText() tcell.Color { return ui.Mix(ui.FgDim, ui.BgTerm, 0.15) }

func (a *App) chromeColors(w int, base tcell.Color) []tcell.Color {
	if !a.st.Running {
		return nil
	}
	a.waveOn.Store(true)
	return ui.WaveColors(w, a.clock().Sub(a.waveEpoch).Seconds(), base, ui.Green)
}

func (a *App) chromeText(c ui.Canvas, x, y, width int, text string, base tcell.Color, colors []tcell.Color) {
	g := uniseg.NewGraphemes(text)
	col := 0
	for g.Next() {
		gw := ui.TextWidth(g.Str())
		if col+gw > width {
			break
		}
		fg := base
		if x+col >= 0 && x+col < len(colors) {
			fg = colors[x+col]
		}
		c.Text(x+col, y, gw, g.Str(), ui.Style{FG: fg, BG: ui.BgTerm}.TCell())
		col += gw
	}
}
