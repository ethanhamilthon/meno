package app

import "meno/internal/ui"

// logText is the plain text of the timeline, one entry per rendered line.
func (a *App) logText() vtext {
	w, _ := a.scr.Size()
	lines := a.transcriptWithSelection(w)
	text := make(vtext, len(lines))
	for i, l := range lines {
		text[i] = []rune(l.Text())
	}
	return text
}

// doc is the whole buffer: timeline lines followed by the input lines. top is
// the number of timeline lines, i.e. the index of the first input line.
func (a *App) doc() (t vtext, top int) {
	lt := a.logText()
	in := a.inputText()
	t = make(vtext, 0, len(lt)+len(in))
	t = append(append(t, lt...), in...)
	return t, len(lt)
}

// curG is the cursor in document coordinates.
func (a *App) curG(top int) vpos {
	if a.st.InLog && top > 0 {
		return vpos{min(a.st.LogLine, top-1), max(0, a.st.LogCol)}
	}
	a.clampCursor()
	p := a.inputText().pos(a.st.Cursor)
	return vpos{top + p.line, p.col}
}

// setCurG moves the cursor to a document position (clamped by the caller).
func (a *App) setCurG(p vpos, top int) {
	if p.line < top {
		a.st.InLog, a.st.LogLine, a.st.LogCol = true, max(0, p.line), max(0, p.col)
		return
	}
	a.st.InLog = false
	in := a.inputText()
	a.st.Cursor = in.idx(in.clamp(vpos{p.line - top, p.col}, true))
}

// anchorG is the visual anchor in document coordinates.
func (a *App) anchorG(top int) vpos {
	if a.st.AnchorLog && top > 0 {
		return vpos{min(a.st.ALine, top-1), a.st.ACol}
	}
	return vpos{top + a.st.ALine, a.st.ACol}
}

func (a *App) setAnchorG(p vpos, top int) {
	if p.line < top {
		a.st.AnchorLog, a.st.ALine, a.st.ACol = true, p.line, p.col
		return
	}
	a.st.AnchorLog, a.st.ALine, a.st.ACol = false, p.line-top, p.col
}

// followCursor scrolls the timeline so that a cursor inside it stays visible.
func (a *App) followCursor() {
	if !a.st.InLog {
		return
	}
	w, h := a.scr.Size()
	n := len(a.transcriptWithSelection(w))
	logH := max(1, h-a.bottomHeight())
	l := min(a.st.LogLine, n-1)
	start := max(0, n-logH-a.st.Scroll)
	switch {
	case l < start:
		a.st.Scroll = n - logH - l
	case l >= start+logH:
		a.st.Scroll = n - 1 - l
	}
	a.st.Scroll = min(max(0, a.st.Scroll), max(0, n-logH))
}

// screenCol is the display offset of rune col inside a rendered timeline line.
func screenCol(line []rune, col int) int {
	col = min(col, len(line))
	return ui.PadX + ui.TextWidth(string(line[:col]))
}
