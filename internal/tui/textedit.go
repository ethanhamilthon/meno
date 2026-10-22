package tui

import (
	"github.com/gdamore/tcell/v2"
)

// textEdit is the text field behind every input outside the composer: form
// fields, searches and renames. It edits like the composer does: the cursor
// moves with the arrow keys, words and line edges, and a multi-line field wraps.
type textEdit struct {
	Text        []rune
	Cursor      int
	Multi       bool   // Enter-with-modifier and Ctrl+J insert a newline
	Secret      bool   // drawn as bullets
	Placeholder string // dim text while the field is empty
	Width       int    // inner width of the last draw; vertical moves wrap at it
	wantCol     int
	wantSet     bool
}

func (t *textEdit) String() string { return string(t.Text) }
func (t *textEdit) Empty() bool    { return len(t.Text) == 0 }

// Set replaces the text and puts the cursor at its end.
func (t *textEdit) Set(s string) {
	t.Text = []rune(s)
	t.Cursor = len(t.Text)
	t.wantSet = false
}

// Reset empties the field and drops the Multi, Secret and Placeholder settings.
func (t *textEdit) Reset() { *t = textEdit{} }

func (t *textEdit) clamp() { t.Cursor = min(max(0, t.Cursor), len(t.Text)) }

// Insert adds text at the cursor. A single-line field turns newlines into spaces.
func (t *textEdit) Insert(rs []rune) {
	t.clamp()
	if !t.Multi {
		rs = append([]rune(nil), rs...)
		for i, r := range rs {
			if r == '\n' || r == '\r' {
				rs[i] = ' '
			}
		}
	}
	out := make([]rune, 0, len(t.Text)+len(rs))
	out = append(out, t.Text[:t.Cursor]...)
	out = append(out, rs...)
	out = append(out, t.Text[t.Cursor:]...)
	t.Text, t.Cursor, t.wantSet = out, t.Cursor+len(rs), false
}

func (t *textEdit) deleteRange(from, to int) {
	if from >= to {
		return
	}
	t.Text = append(append([]rune(nil), t.Text[:from]...), t.Text[to:]...)
	t.Cursor, t.wantSet = from, false
}

// Handle applies an editing or cursor key and reports whether it was one.
// Enter, Esc and Tab are never handled (the caller commits, cancels or moves
// focus), except Enter with a modifier in a multi-line field.
func (t *textEdit) Handle(k *tcell.EventKey) bool {
	t.clamp()
	mods := k.Modifiers()
	word := mods&(tcell.ModAlt|tcell.ModCtrl) != 0
	c := t.Cursor
	switch k.Key() {
	case tcell.KeyEnter:
		if t.Multi && mods&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0 {
			t.Insert([]rune{'\n'})
			return true
		}
		return false
	case tcell.KeyCtrlJ:
		if !t.Multi {
			return false
		}
		t.Insert([]rune{'\n'})
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if mods&tcell.ModAlt != 0 {
			t.deleteRange(wordBoundary(t.Text, c, -1), c)
		} else if c > 0 {
			t.deleteRange(c-lastGraphemeLen(t.Text[:c]), c)
		}
	case tcell.KeyDelete:
		if c < len(t.Text) {
			t.deleteRange(c, c+firstGraphemeLen(t.Text[c:]))
		}
	case tcell.KeyCtrlW:
		t.deleteRange(wordBoundary(t.Text, c, -1), c)
	case tcell.KeyCtrlU:
		t.deleteRange(lineEdge(t.Text, c, -1), c)
	case tcell.KeyCtrlK:
		end := lineEdge(t.Text, c, 1)
		if end == c && end < len(t.Text) {
			end++ // at the end of a line: join the next one
		}
		t.deleteRange(c, end)
	case tcell.KeyLeft:
		if word {
			t.Cursor = wordBoundary(t.Text, c, -1)
		} else if c > 0 {
			t.Cursor = c - lastGraphemeLen(t.Text[:c])
		}
		t.wantSet = false
	case tcell.KeyRight:
		if word {
			t.Cursor = wordBoundary(t.Text, c, 1)
		} else if c < len(t.Text) {
			t.Cursor = c + firstGraphemeLen(t.Text[c:])
		}
		t.wantSet = false
	case tcell.KeyUp, tcell.KeyDown:
		if !t.Multi {
			return false
		}
		d := 1
		if k.Key() == tcell.KeyUp {
			d = -1
		}
		t.moveVert(d)
	case tcell.KeyHome, tcell.KeyCtrlA:
		t.Cursor, t.wantSet = lineEdge(t.Text, c, -1), false
	case tcell.KeyEnd, tcell.KeyCtrlE:
		t.Cursor, t.wantSet = lineEdge(t.Text, c, 1), false
	case tcell.KeyRune:
		switch {
		case mods&tcell.ModAlt != 0 && k.Rune() == 'b':
			t.Cursor, t.wantSet = wordBoundary(t.Text, c, -1), false
		case mods&tcell.ModAlt != 0 && k.Rune() == 'f':
			t.Cursor, t.wantSet = wordBoundary(t.Text, c, 1), false
		case mods&(tcell.ModCtrl|tcell.ModAlt) != 0:
			return false
		default:
			t.Insert([]rune{k.Rune()})
		}
	default:
		return false
	}
	return true
}

func (t *textEdit) moveVert(d int) {
	w := t.Width
	if w <= 0 {
		w = 40
	}
	rows := layoutInput(t.Text, w)
	if !t.wantSet {
		r := cursorRow(rows, t.Cursor)
		t.wantCol, t.wantSet = widthBetween(t.Text, rows[r].start, t.Cursor), true
	}
	t.Cursor = vertTarget(t.Text, rows, t.Cursor, t.wantCol, d)
}
