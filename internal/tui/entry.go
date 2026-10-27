package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// An entry is the main composer editing a short text on behalf of a dialog: a
// search, a setting, a field of a form, a step of the add wizard. The user's own
// draft is put aside while the entry lasts and comes back unchanged. Nothing
// else edits text: there is no second editor.
type entry struct {
	label  string             // drawn before the text
	secret bool               // the text is drawn as bullets
	hint   string             // dim text while empty
	done   func(string) error // Enter; an error is shown and the entry stays. nil: a live entry
	cancel func()             // Esc of a modal entry
	rt     *Runtime           // the session whose draft is put aside
	aside  asideDraft
}

// asideDraft is the composer of a session while an entry uses it.
type asideDraft struct {
	images     []draftImage
	input      []rune
	cursor     int
	wantCol    int
	wantSet    bool
	top        int
	undo, redo []snap
	base       snap
}

// beginEntry puts the draft aside and starts editing initial. A live entry (no
// done) leaves the keys to the dialog, which calls entryEdit for text keys; a
// modal entry owns Enter and Esc as well.
func (a *App) beginEntry(e entry, initial string) {
	a.endEntry()
	rt := a.st.Runtime
	e.rt = rt
	e.aside = asideDraft{rt.Images, rt.Input, rt.Cursor, rt.WantCol, rt.WantSet, rt.InputTop, rt.Undo, rt.Redo, rt.InsBase}
	a.st.Entry = &e
	rt.Images, rt.InLog, rt.InputTop = nil, false, 0
	a.setEntryText(initial)
	rt.Undo, rt.Redo, rt.InsBase = nil, nil, rt.snapshot()
}

// endEntry stops the entry and gives the draft back.
func (a *App) endEntry() {
	e := a.st.Entry
	if e == nil {
		return
	}
	a.st.Entry = nil
	rt, d := e.rt, e.aside
	rt.Images, rt.Input, rt.Cursor, rt.WantCol, rt.WantSet, rt.InputTop = d.images, d.input, d.cursor, d.wantCol, d.wantSet, d.top
	rt.Undo, rt.Redo, rt.InsBase = d.undo, d.redo, d.base
	rt.clampCursor()
}

// entryText is the text being edited, "" when there is no entry.
func (a *App) entryText() string {
	if e := a.st.Entry; e != nil {
		return string(e.rt.Input)
	}
	return ""
}

// setEntryText replaces the text of the entry and puts the cursor at its end.
func (a *App) setEntryText(s string) {
	if e := a.st.Entry; e != nil {
		e.rt.Input, e.rt.Cursor, e.rt.WantSet = []rune(s), len([]rune(s)), false
	}
}

// query is the search text of the open list dialog.
func (a *App) query() string { return a.entryText() }

// setQuery replaces the search text of the open list dialog.
func (a *App) setQuery(s string) { a.setEntryText(s) }

// entryKey handles a key of a modal entry: Enter confirms, Esc cancels.
func (a *App) entryKey(k *tcell.EventKey) {
	e := a.st.Entry
	if e.rt != a.st.Runtime { // another session came on screen: the entry is over
		a.endEntry()
		return
	}
	switch k.Key() {
	case tcell.KeyEsc:
		if e.cancel != nil {
			e.cancel()
		}
		if a.st.Entry == e {
			a.endEntry()
		}
	case tcell.KeyEnter:
		if err := e.done(a.entryText()); err != nil {
			a.notifyErr(err.Error())
			return
		}
		if a.st.Entry == e { // done may have started the next entry
			a.endEntry()
		}
	default:
		a.entryEdit(k, false)
	}
}

// entryEdit applies a text key to the composer and reports whether the text
// changed. Newlines are for multi-line answers only; a single-line entry never
// gets one.
func (a *App) entryEdit(k *tcell.EventKey, multi bool) bool {
	a.clampCursor()
	before := string(a.st.Input)
	word := k.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) != 0
	switch k.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.backspace()
	case tcell.KeyDelete:
		a.deleteForward()
	case tcell.KeyLeft:
		if word {
			a.moveWord(-1)
		} else {
			a.moveLeft()
		}
	case tcell.KeyRight:
		if word {
			a.moveWord(1)
		} else {
			a.moveRight()
		}
	case tcell.KeyUp, tcell.KeyDown:
		if multi {
			d := 1
			if k.Key() == tcell.KeyUp {
				d = -1
			}
			a.moveVert(d)
		}
	case tcell.KeyHome, tcell.KeyCtrlA:
		a.moveLineEdge(-1)
	case tcell.KeyEnd, tcell.KeyCtrlE:
		a.moveLineEdge(1)
	case tcell.KeyCtrlU:
		a.deleteRange(0, len(a.st.Input))
	case tcell.KeyCtrlZ:
		a.endInsert()
		a.undo()
	case tcell.KeyCtrlR:
		a.redo()
	case tcell.KeyCtrlJ:
		if multi {
			a.insertNewline()
		}
	case tcell.KeyEnter:
		if multi {
			a.insertNewline()
		}
	case tcell.KeyRune:
		if !isObjRune(k.Rune()) {
			a.insertAtCursor([]rune{k.Rune()})
		}
	}
	return string(a.st.Input) != before
}

// entryPaste inserts pasted text into the entry as one line.
func (a *App) entryPaste(text string) {
	text = strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
	if text == "" {
		return
	}
	a.clampCursor()
	a.insertAtCursor([]rune(text))
	if e := a.st.Entry; e != nil && e.done == nil {
		a.st.Sel = 0 // a live search starts from its first row
	}
}

// drawEntryComposer draws the input band of an entry: the label, then the text
// scrolled sideways to keep the cursor in view. The cursor is a reversed cell.
func (a *App) drawEntryComposer(c render.Canvas, x, y, w, h int) bool {
	e := a.st.Entry
	if e == nil {
		return false
	}
	c.Fill(x, y, w, h, render.BgPopup)
	row := y + min(1, max(0, h-1))
	pad := min(render.Gutter, max(0, (w-1)/2))
	width := max(0, w-2*pad)
	used := c.Text(x+pad, row, width, e.label, render.Style{FG: render.Blue, BG: render.BgPopup}.TCell())
	tx, tw := x+pad+used, max(0, width-used)
	if tw <= 0 {
		return true
	}
	st := render.Style{FG: render.FgWhite, BG: render.BgPopup}.TCell()
	in := a.st.Input
	if e.secret {
		in = []rune(strings.Repeat("•", len(in)))
	}
	cur := min(max(0, a.st.Cursor), len(in))
	if len(in) == 0 {
		hint := st.Foreground(render.FgDim)
		c.Text(tx, row, tw, e.hint, hint)
		r := ' '
		if p := []rune(e.hint); len(p) > 0 {
			r = p[0]
		}
		c.S.SetContent(tx, row, r, nil, hint.Reverse(true))
		return true
	}
	start := 0
	for widthBetween(in, start, cur) > tw-1 {
		start += firstGraphemeLen(in[start:])
	}
	c.Text(tx, row, tw, string(in[start:]), st)
	r := ' '
	if cur < len(in) && in[cur] != '\n' {
		r = in[cur]
	}
	c.S.SetContent(tx+widthBetween(in, start, cur), row, r, nil, st.Reverse(true))
	return true
}
