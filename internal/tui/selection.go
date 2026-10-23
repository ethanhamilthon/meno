package tui

// A selection is made with the mouse: a drag, a double click (word) or a triple
// click (line). It is copied to the clipboard on release and stays highlighted
// until the next key or click. The anchor and the end are document positions
// (see doc.go) in the timeline or in the composer.

// startSelection turns the anchor and the current end into a selection.
func (a *App) startSelection(line bool) {
	a.st.Selecting, a.st.SelLine = true, line
}

// clearSelection drops the highlight and returns the cursor to the composer.
func (a *App) clearSelection() {
	if !a.st.Selecting {
		return
	}
	a.st.Selecting, a.st.SelLine = false, false
	a.st.InLog = false
	a.clampCursor()
}

// copyText puts text on the system clipboard and remembers it.
func (a *App) copyText(text string) {
	a.st.Copied = text
	if text != "" {
		a.scr.SetClipboard([]byte(externalText([]rune(text))))
	}
}

// selOrder returns the selection ends in document order.
func (a *App) selOrder(top int) (s, e vpos) {
	s, e = a.anchorG(top), a.curG(top)
	if e.less(s) {
		s, e = e, s
	}
	return
}

// selRng turns the selection into one range of the document.
func (a *App) selRng(t vtext, top int) rng {
	s, e := a.selOrder(top)
	return t.span(s, e, a.st.SelLine)
}

// selSpan reports the selected rune columns [lo, hi) of document line l (n is
// its length); ok is false when the line is not selected at all.
func (a *App) selSpan(top, l, n int) (lo, hi int, ok bool) {
	if !a.st.Selecting {
		return 0, 0, false
	}
	s, e := a.selOrder(top)
	if l < s.line || l > e.line {
		return 0, 0, false
	}
	if a.st.SelLine {
		return 0, n, true
	}
	lo, hi = 0, n
	if l == s.line {
		lo = s.col
	}
	if l == e.line {
		hi = min(n, e.col+1)
	}
	return lo, hi, true
}

// selectedText is the text under the selection.
func (a *App) selectedText() string {
	t, top := a.doc()
	if len(t) == 0 {
		return ""
	}
	return t.slice(a.selRng(t, top))
}
