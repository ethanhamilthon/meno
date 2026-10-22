package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// shown is the text as drawn: bullets for a secret field, one per rune so that
// cursor offsets stay valid.
func (t *textEdit) shown() []rune {
	if !t.Secret {
		return t.Text
	}
	return []rune(strings.Repeat("•", len(t.Text)))
}

// Draw paints the field into the w×h cells at (x, y). A single row scrolls
// sideways to keep the cursor in view; more rows wrap a multi-line field. The
// cursor is a reversed cell, shown only while focused.
func (t *textEdit) Draw(c render.Canvas, x, y, w, h int, st tcell.Style, focused bool) {
	if w <= 0 || h <= 0 {
		return
	}
	t.clamp()
	t.Width = w
	if t.Empty() {
		c.Text(x, y, w, t.Placeholder, st.Foreground(render.FgDim))
		if focused { // the cursor sits on the first letter of the hint
			r := ' '
			if p := []rune(t.Placeholder); len(p) > 0 {
				r = p[0]
			}
			t.drawCursor(c, x, y, r, st.Foreground(render.FgDim))
		}
		return
	}
	in := t.shown()
	if h == 1 || !t.Multi {
		start := 0
		for widthBetween(in, start, t.Cursor) > w-1 {
			start += firstGraphemeLen(in[start:])
		}
		c.Text(x, y, w, string(in[start:]), st)
		if focused {
			t.drawCursorAt(c, x+widthBetween(in, start, t.Cursor), y, in, st)
		}
		return
	}
	rows := layoutInput(in, w)
	cr := cursorRow(rows, t.Cursor)
	top := max(0, cr-h+1)
	for i := 0; i < h && top+i < len(rows); i++ {
		row := rows[top+i]
		c.Text(x, y+i, w, string(in[row.start:row.end]), st)
	}
	if focused {
		row := rows[cr]
		t.drawCursorAt(c, x+widthBetween(in, row.start, t.Cursor), y+cr-top, in, st)
	}
}

func (t *textEdit) drawCursorAt(c render.Canvas, x, y int, in []rune, st tcell.Style) {
	r := ' '
	if t.Cursor < len(in) && in[t.Cursor] != '\n' {
		r = in[t.Cursor]
	}
	t.drawCursor(c, x, y, r, st)
}

func (t *textEdit) drawCursor(c render.Canvas, x, y int, r rune, st tcell.Style) {
	c.S.SetContent(x, y, r, nil, st.Reverse(true))
}
