package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// paintBG recolors the background of w cells, keeping text and attributes.
func paintBG(c render.Canvas, x, y, w int, bg tcell.Color) {
	sw, sh := c.S.Size()
	for i := 0; i < w; i++ {
		if x+i < 0 || x+i >= sw || y < 0 || y >= sh {
			continue
		}
		r, comb, st, _ := c.S.GetContent(x+i, y)
		c.S.SetContent(x+i, y, r, comb, st.Background(bg))
	}
}

// paintLog highlights the selection in the visible timeline rows. start is the
// index of the first visible line.
func (a *App) paintLog(c render.Canvas, x, y, w, h int, lines []render.Line, start int) {
	top := len(lines)
	for i := 0; i < h && start+i < len(lines); i++ {
		l := start + i
		rs := []rune(lines[l].Text())
		if lo, hi, ok := a.selSpan(top, l, len(rs)); ok {
			if a.st.SelLine {
				paintBG(c, x, y+i, w, render.BgSelect)
			} else if len(rs) == 0 {
				paintBG(c, x+render.PadX, y+i, 1, render.BgSelect)
			} else {
				from := screenCol(rs, lo)
				paintBG(c, x+from, y+i, screenCol(rs, hi)-from, render.BgSelect)
			}
		}
	}
}

// inputSelMask marks the input runes covered by the selection.
func (a *App) inputSelMask() []bool {
	if !a.st.Selecting {
		return nil
	}
	top := len(a.logText())
	mask := make([]bool, len(a.st.Input)+1)
	off := 0
	for i, l := range a.inputText() {
		if lo, hi, ok := a.selSpan(top, top+i, len(l)); ok {
			for j := lo; j < hi && off+j < len(mask); j++ {
				mask[off+j] = true
			}
		}
		off += len(l) + 1
	}
	return mask
}
