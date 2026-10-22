package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"

	"meno/internal/render"
)

// drawComposer draws the input band. Text starts below the top padding; the
// real terminal cursor marks the edit position (see applyCursor).
func (a *App) drawComposer(c render.Canvas, x, y, w, h int) {
	pad := min(composerPadX, max(0, (w-1)/2))
	textX := x + pad
	innerW := max(0, w-2*pad)
	topPad := min(1, max(0, h-1))
	bottomPad := min(composerPadY, max(0, h-topPad-1))
	row := y + topPad
	bg := render.BgPopup
	defer a.paintComposerBackground(c, x, y, w, h)
	a.clampCursor()
	if len(a.st.Input) == 0 {
		a.st.ComposerRows = []vrow{{0, 0, false}}
		a.st.ComposerX, a.st.ComposerY, a.st.ComposerTop = textX, row, 0
		if !a.st.InLog {
			a.cur = cursorPos{textX, row, true}
		}
		hint := a.placeholder()
		c.Text(textX, row, innerW, hint, render.Style{FG: chromeText(), BG: bg}.TCell())
		return
	}
	rows := layoutInput(a.st.Input, max(1, innerW))
	avail := max(1, h-topPad-bottomPad)
	cr := cursorRow(rows, a.st.Cursor)
	top := min(max(0, a.st.InputTop), max(0, len(rows)-avail))
	if cr < top {
		top = cr
	}
	if cr >= top+avail {
		top = cr - avail + 1
	}
	a.st.InputTop = top
	a.st.ComposerRows, a.st.ComposerX, a.st.ComposerY, a.st.ComposerTop = rows, textX, row, top
	toks, mask := a.inputTokens(), a.inputSelMask()
	for i := 0; i < avail && top+i < len(rows); i++ {
		a.drawInputRow(c, textX, row+i, innerW, rows[top+i], toks, bg, mask)
	}
	col := widthBetween(a.st.Input, rows[cr].start, a.st.Cursor)
	if !a.st.InLog {
		a.cur = cursorPos{textX + min(col, max(0, innerW-1)), row + cr - top, true}
	}
}

// drawInputRow draws input[row.start:row.end] with prompt/skill highlighting.
func (a *App) drawInputRow(c render.Canvas, x, y, maxW int, row vrow, toks []tokSpan, bg tcell.Color, mask []bool) {
	in := a.st.Input
	plain := render.Style{FG: render.FgWhite, BG: bg}.TCell()
	col, idx := 0, row.start
	g := uniseg.NewGraphemes(string(in[row.start:row.end]))
	for g.Next() && col < maxW {
		s := g.Str()
		st := plain
		for _, t := range toks {
			if idx >= t.start && idx < t.end {
				st = render.Style{FG: tokenColor(t.kind), BG: bg}.TCell()
				break
			}
		}
		if idx < len(mask) && mask[idx] {
			st = st.Background(render.BgSelect)
		}
		if s == "\t" {
			s = "    "
		}
		if r := []rune(s); len(r) == 1 {
			if o, ok := objectAt(r[0]); ok {
				s = o.Label()
				st = render.Style{FG: render.Blue, BG: bg}.TCell()
				if idx < len(mask) && mask[idx] {
					st = st.Background(render.BgSelect)
				}
			}
		}
		col += c.Text(x+col, y, maxW-col, s, st)
		idx += len([]rune(g.Str()))
	}
}
