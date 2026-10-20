package app

import (
	"github.com/gdamore/tcell/v2"
	"slices"
)

type frameCell struct {
	glyph     rune
	combining string
	style     tcell.Style
	width     int
}
type frameState struct {
	cells  []frameCell
	w, h   int
	cursor cursorPos
	mode   int
	hidden bool
}

// changedFrame avoids terminal hide/show cursor sequences for identical frames.
func (a *App) changedFrame(w, h int) bool {
	frame := frameState{w: w, h: h, cursor: a.cur, mode: int(a.st.Mode), hidden: a.st.Over != OverNone || a.askChoosing()}
	frame.cells = make([]frameCell, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			glyph, combining, style, width := a.scr.GetContent(x, y)
			frame.cells = append(frame.cells, frameCell{glyph, string(combining), style, width})
		}
	}
	old := a.lastFrame
	changed := old.w != w || old.h != h || old.cursor != frame.cursor || old.mode != frame.mode || old.hidden != frame.hidden || !slices.Equal(old.cells, frame.cells)
	a.lastFrame = frame
	return changed
}
