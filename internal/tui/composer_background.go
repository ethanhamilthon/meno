package tui

import "meno/internal/render"

// paintComposerBackground applies one field across padding and glyphs. Selection
// keeps its own background; foregrounds, grapheme clusters and attributes survive.
func (a *App) paintComposerBackground(c render.Canvas, x, y, w, h int) {
	colors := make([]float64, w)
	if a.st.Running {
		a.waveOn.Store(true)
		colors = render.WaveIntensity(w, a.clock().Sub(a.waveEpoch).Seconds())
	}
	sw, sh := c.S.Size()
	for row := max(0, y); row < min(sh, y+h); row++ {
		for col := max(0, x); col < min(sw, x+w); {
			glyph, comb, style, width := c.S.GetContent(col, row)
			_, bg, _ := style.Decompose()
			if bg == render.BgSelect {
				col += max(1, width)
				continue
			}
			bg = render.Mix(render.BgPopup, render.Green, 0.20*colors[col-x])
			c.S.SetContent(col, row, glyph, comb, style.Background(bg))
			col += max(1, width)
		}
	}
}
