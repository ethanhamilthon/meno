package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// paintBG recolors the background of w cells, keeping text and attributes.
func paintBG(c ui.Canvas, x, y, w int, bg tcell.Color) {
	sw, sh := c.S.Size()
	for i := 0; i < w; i++ {
		if x+i < 0 || x+i >= sw || y < 0 || y >= sh {
			continue
		}
		r, comb, st, _ := c.S.GetContent(x+i, y)
		c.S.SetContent(x+i, y, r, comb, st.Background(bg))
	}
}

// paintLog highlights the visual selection in the visible timeline rows and
// places the cursor when it is inside the timeline. start is the index of the
// first visible line.
func (a *App) paintLog(c ui.Canvas, x, y, w, h int, lines []ui.Line, start int) {
	top := len(lines)
	mode := a.st.Mode
	for i := 0; i < h && start+i < len(lines); i++ {
		l := start + i
		rs := []rune(lines[l].Text())
		if lo, hi, ok := a.selSpan(top, l, len(rs)); ok {
			if a.st.VKind == 'V' {
				paintBG(c, x, y+i, w, ui.BgSelect)
			} else if len(rs) == 0 {
				paintBG(c, x+ui.PadX, y+i, 1, ui.BgSelect)
			} else {
				from := screenCol(rs, lo)
				paintBG(c, x+from, y+i, screenCol(rs, hi)-from, ui.BgSelect)
			}
		}
		if a.st.InLog && l == a.st.LogLine && (mode == ui.ModeInput || mode == ui.ModeVisual || mode == ui.ModeEdit) {
			cx := screenCol(rs, a.st.LogCol)
			if cx < w {
				a.cur = cursorPos{x + cx, y + i, true}
			}
		}
	}
}

// inputSelMask marks the input runes covered by the visual selection.
func (a *App) inputSelMask() []bool {
	if a.st.Mode != ui.ModeVisual {
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
