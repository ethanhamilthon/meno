package app

import (
	"github.com/rivo/uniseg"
)

// vrow is one visual composer row: input[start:end] (rune offsets). soft marks a
// row that ended because of wrapping rather than a newline or the end of input.
type vrow struct {
	start, end int
	soft       bool
}

func cellWidth(g string) int {
	if g == "\t" {
		return 4
	}
	if len(g) == 3 { // an object is one private-use rune (3 bytes in UTF-8)
		if r := []rune(g); len(r) == 1 {
			if o, ok := objectAt(r[0]); ok {
				return uniseg.StringWidth(o.Label())
			}
		}
	}
	return uniseg.StringWidth(g)
}

// cursorRow is the row holding the cursor: the last row starting at or before it.
func cursorRow(rows []vrow, cursor int) int {
	r := 0
	for i, row := range rows {
		if row.start <= cursor {
			r = i
		}
	}
	return r
}

// widthBetween is the display width of in[from:to].
func widthBetween(in []rune, from, to int) int {
	w := 0
	g := uniseg.NewGraphemes(string(in[from:to]))
	for g.Next() {
		w += cellWidth(g.Str())
	}
	return w
}

func lastGraphemeLen(rs []rune) int {
	n := 0
	g := uniseg.NewGraphemes(string(rs))
	for g.Next() {
		n = len([]rune(g.Str()))
	}
	return n
}

func firstGraphemeLen(rs []rune) int {
	g := uniseg.NewGraphemes(string(rs))
	if g.Next() {
		return len([]rune(g.Str()))
	}
	return 0
}

func (a *App) clampCursor() { a.st.Runtime.clampCursor() }

// setInput replaces the active composer and puts the cursor at its end.
func (a *App) setInput(rs []rune) { a.setInputFor(a.st.Runtime, rs) }

func (a *App) insertAtCursor(rs []rune) { a.insertAtCursorFor(a.st.Runtime, rs) }

func (a *App) insertNewline() { a.insertAtCursor([]rune{'\n'}) }

func (a *App) deleteRange(from, to int) {
	if from >= to {
		return
	}
	out := make([]rune, 0, len(a.st.Input)-(to-from))
	out = append(out, a.st.Input[:from]...)
	out = append(out, a.st.Input[to:]...)
	a.st.Input = out
	a.st.Cursor = from
	a.st.WantSet = false
	a.st.Sel = 0
}

func (a *App) backspace() {
	a.clampCursor()
	c := a.st.Cursor
	if c == 0 {
		return
	}
	if s, ok := a.imageEndingAt(c); ok {
		a.deleteRange(s.start, s.end)
		a.pruneImages()
		return
	}
	a.deleteRange(c-lastGraphemeLen(a.st.Input[:c]), c)
}

func (a *App) deleteForward() {
	a.clampCursor()
	c := a.st.Cursor
	if c >= len(a.st.Input) {
		return
	}
	if s, ok := a.imageStartingAt(c); ok {
		a.deleteRange(s.start, s.end)
		a.pruneImages()
		return
	}
	a.deleteRange(c, c+firstGraphemeLen(a.st.Input[c:]))
}

func (a *App) moveLeft() {
	a.clampCursor()
	if c := a.st.Cursor; c > 0 {
		if s, ok := a.imageEndingAt(c); ok {
			a.st.Cursor = s.start
		} else {
			a.st.Cursor = c - lastGraphemeLen(a.st.Input[:c])
		}
	}
	a.st.WantSet, a.st.Sel = false, 0
}

func (a *App) moveRight() {
	a.clampCursor()
	if c := a.st.Cursor; c < len(a.st.Input) {
		if s, ok := a.imageStartingAt(c); ok {
			a.st.Cursor = s.end
		} else {
			a.st.Cursor = c + firstGraphemeLen(a.st.Input[c:])
		}
	}
	a.st.WantSet, a.st.Sel = false, 0
}

// moveWord jumps to the previous (dir<0) or next (dir>0) word boundary.
func (a *App) moveWord(dir int) {
	a.clampCursor()
	a.st.Cursor, a.st.WantSet, a.st.Sel = wordBoundary(a.st.Input, a.st.Cursor, dir), false, 0
}

// moveLineEdge moves to the start (dir<0) or end (dir>0) of the logical line.
func (a *App) moveLineEdge(dir int) {
	a.clampCursor()
	a.st.Cursor, a.st.WantSet, a.st.Sel = lineEdge(a.st.Input, a.st.Cursor, dir), false, 0
}

// moveVert moves the cursor one visual row up (d<0) or down (d>0), keeping the
// column. On the first/last row it jumps to the start/end of the input.
func (a *App) moveVert(d int) {
	a.clampCursor()
	w, _ := a.scr.Size()
	in := a.st.Input
	rows := layoutInput(in, a.composerInnerWidth(w))
	if !a.st.WantSet {
		r := cursorRow(rows, a.st.Cursor)
		a.st.WantCol, a.st.WantSet = widthBetween(in, rows[r].start, a.st.Cursor), true
	}
	a.st.Sel = 0
	a.st.Cursor = vertTarget(in, rows, a.st.Cursor, a.st.WantCol, d)
}
