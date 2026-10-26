package tui

import "meno/internal/render"

// logText is the plain text of the timeline, one entry per rendered line.
func (a *App) logText() vtext {
	w, _ := a.scr.Size()
	lines := a.transcriptWithSelection(w)
	c := &a.st.transcript
	if c.text != nil && c.textStamp == c.stamp {
		return c.text
	}
	text := make(vtext, len(lines))
	for i, l := range lines {
		text[i] = []rune(l.Text())
	}
	c.text, c.textStamp = text, c.stamp
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

// screenCol is the display offset of rune col inside a rendered timeline line.
func screenCol(line []rune, col int) int {
	col = min(col, len(line))
	return render.PadX + render.TextWidth(string(line[:col]))
}
