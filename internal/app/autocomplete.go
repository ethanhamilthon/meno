package app

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

const (
	acMax       = 25 // most rows a completion list may occupy
	acTwoColMin = 6  // from this many items on the list uses two columns
	acTwoColW   = 44 // narrower terminals keep a single column
)

// acCell is one entry of a completion list.
type acCell struct {
	label, desc string
	labelFG     tcell.Color // label color of the selected entry
}

// acColumns is 1 for short lists and narrow terminals, otherwise 2.
func acColumns(count, w int) int {
	if count < acTwoColMin || w < acTwoColW {
		return 1
	}
	return 2
}

// acRowCount is the number of rows a completion list of count items needs at
// width w: one item per row up to five items, then two columns filled top to
// bottom (the left column first), at most acMax rows.
func acRowCount(count, w int) int {
	if count <= 0 {
		return 0
	}
	cols := acColumns(count, w)
	return min(acMax, (count+cols-1)/cols)
}

// drawAcGrid draws a completion list in the rows y..y+limit. Entries run down
// the left column and continue at the top of the right one, so plain up/down
// walks through all of them. Longer lists scroll page by page.
func drawAcGrid(c ui.Canvas, x, y, w, limit int, cells []acCell, sel int) {
	n := len(cells)
	rows := min(acRowCount(n, w), max(0, limit))
	if rows == 0 || w <= 0 {
		return
	}
	cols := acColumns(n, w)
	perPage := rows * cols
	sel = clampSel(sel, n)
	start := sel / perPage * perPage
	c.Fill(x, y, w, rows, ui.BgPopup)
	for idx := 0; idx < perPage && start+idx < n; idx++ {
		it := cells[start+idx]
		cw := w / cols
		cx := x + idx/rows*cw
		if idx/rows == cols-1 {
			cw = w - cw*(cols-1)
		}
		row := y + idx%rows
		bg, fg, mark := ui.BgPopup, ui.FgSoft, " "
		if start+idx == sel {
			bg, fg, mark = ui.BgPopupSel, it.labelFG, "▸"
		}
		c.Fill(cx, row, cw, 1, bg)
		c.Text(cx+2, row, 2, mark, tcell.StyleDefault.Foreground(ui.Blue).Background(bg))
		labelW := min(24, max(6, cw*2/5))
		c.Text(cx+4, row, labelW, it.label, tcell.StyleDefault.Foreground(fg).Background(bg))
		c.Text(cx+4+labelW+1, row, max(0, cw-labelW-6-ui.Gutter), oneLine(it.desc), tcell.StyleDefault.Foreground(ui.FgDim).Background(bg))
	}
}

func (a *App) acActive() bool {
	if a.st.AcDismiss != "" && a.st.AcDismiss == a.acIdentity() {
		return false
	}
	return a.st.Ask == nil && a.st.Mode == ui.ModeInput && len(a.acMatches()) > 0
}

// acAccept replaces the word under the cursor with the selected completion.
func (a *App) acAccept() {
	t, ok := a.acToken()
	m := a.acMatches()
	if !ok || len(m) == 0 {
		return
	}
	it := m[clampSel(a.st.Sel, len(m))]
	in := a.st.Input
	rep := []rune(it.label())
	if k, ok := objectKind(it.kind); ok {
		// An explicit selection is one atomic object: a skill pinned, or a mode named.
		if r, ok := objectRune(inputObject{Kind: k, Ref: it.name}); ok {
			rep = []rune{r}
		}
	}
	tail := in[t.end:]
	out := make([]rune, 0, len(in)+len(rep)+1)
	out = append(out, in[:t.start]...)
	out = append(out, rep...)
	cursor := len(out)
	switch {
	case len(tail) == 0 || !unicode.IsSpace(tail[0]):
		out = append(out, ' ')
		cursor++
	case tail[0] == ' ':
		cursor++
	}
	out = append(out, tail...)
	a.st.Input, a.st.Cursor, a.st.WantSet, a.st.Sel = out, cursor, false, 0
}

// acKey handles keys while the completion popup is open. It returns true if consumed.
func (a *App) acKey(k *tcell.EventKey) bool {
	if a.pickOpen() {
		return a.pickKey(k)
	}
	m := a.acMatches()
	n := len(m)
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.AcDismiss = a.acIdentity()
		return true
	case tcell.KeyDown:
		a.st.Sel = clampSel(a.st.Sel+1, n)
		return true
	case tcell.KeyUp:
		a.st.Sel = clampSel(a.st.Sel-1, n)
		return true
	case tcell.KeyTab:
		a.acAccept()
		return true
	case tcell.KeyEnter:
		if k.Modifiers()&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0 {
			return false // newline
		}
		// A reference that is already complete should be sent, not re-completed.
		if t, ok := a.acToken(); ok && n > 0 {
			it := m[clampSel(a.st.Sel, n)]
			if string(a.st.Input[t.start:t.end]) == it.label() {
				return false
			}
		}
		a.acAccept()
		return true
	}
	return false
}

func (a *App) acHeight() int {
	if a.st.Ask != nil {
		return a.askHeight()
	}
	if !a.acActive() {
		return a.queueHeight()
	}
	w, _ := a.scr.Size()
	return acRowCount(len(a.acMatches()), w)
}

func (a *App) drawAutocompleteRows(c ui.Canvas, x, y, w, limit int) {
	m := a.acMatches()
	cells := make([]acCell, len(m))
	for i, it := range m {
		cells[i] = acCell{it.label(), it.desc, tokenColor(it.kind)}
	}
	drawAcGrid(c, x, y, w, limit, cells, a.st.Sel)
}

func tokenColor(kind rune) tcell.Color {
	switch kind {
	case kindSkill:
		return ui.Blue
	}
	return ui.Purple
}

func (a *App) acIdentity() string {
	token, ok := a.acToken()
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%s", token.start, token.end, a.st.Cursor, string(a.st.Input))
}
