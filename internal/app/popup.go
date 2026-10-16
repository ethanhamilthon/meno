package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type box struct{ x, y, w, h int }

func centered(sw, sh, w, h int) box {
	w = min(w, sw)
	h = min(h, sh)
	return box{(sw - w) / 2, (sh - h) / 2, w, h}
}

func (b box) bg(sel bool) tcell.Color {
	if sel {
		return ui.BgPopupSel
	}
	return ui.BgPopup
}

func (b box) fill(c ui.Canvas, y int, bg tcell.Color) {
	c.Fill(b.x, b.y+y, b.w, 1, bg)
}

func (b box) put(c ui.Canvas, x, y int, text string, fg, bg tcell.Color) int {
	return c.Text(b.x+x, b.y+y, b.w-x, text, tcell.StyleDefault.Foreground(fg).Background(bg))
}

func (b box) putRight(c ui.Canvas, y int, text string, fg, bg tcell.Color) {
	x := max(0, b.w-2-ui.TextWidth(text))
	b.put(c, x, y, text, fg, bg)
}

func (b box) hint(c ui.Canvas, text string) {
	b.fill(c, b.h-1, ui.BgPopup)
	b.put(c, 2, b.h-1, text, ui.FgDim, ui.BgPopup)
}

func (b box) frame(c ui.Canvas) {
	c.Fill(b.x, b.y, b.w, b.h, ui.BgPopup)
}

type item struct {
	glyph   string
	glyphFG tcell.Color
	label   string
	accent  bool
	right   string
	detail  string
}

func itemsHeight(items []item, sel int) int {
	n := len(items)
	if sel >= 0 && sel < len(items) && items[sel].detail != "" {
		n++
	}
	return n
}

func drawItems(c ui.Canvas, b box, y int, items []item, sel int) {
	for i, it := range items {
		isSel := i == sel
		bg := b.bg(isSel)
		b.fill(c, y, bg)
		mark := " "
		if isSel {
			mark = "▸"
		}
		b.put(c, 2, y, mark, ui.Blue, bg)
		x := 4
		if it.glyph != "" {
			x += b.put(c, x, y, it.glyph+" ", it.glyphFG, bg)
		}
		lfg := ui.FgSoft
		if it.accent {
			lfg = ui.Blue
		}
		if isSel {
			lfg = ui.FgWhite
		}
		b.put(c, x, y, it.label, lfg, bg)
		if it.right != "" {
			b.putRight(c, y, it.right, ui.FgDim, bg)
		}
		y++
		if isSel && it.detail != "" {
			b.fill(c, y, bg)
			b.put(c, 4, y, it.detail, ui.FgDim, bg)
			y++
		}
	}
}

type headerFn func(c ui.Canvas, b box)

func titleRow(title, sub string) headerFn {
	return func(c ui.Canvas, b box) {
		b.put(c, 2, 0, title, ui.Blue, ui.BgPopup)
		if sub != "" {
			b.putRight(c, 0, sub, ui.FgDim, ui.BgPopup)
		}
	}
}

func listBox(c ui.Canvas, w, h, bw int, header headerFn, items []item, sel int, hint string) box {
	bh := 4 + itemsHeight(items, sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, bw, bh)
	b.frame(c)
	header(c, b)
	if len(items) == 0 {
		b.put(c, 4, 2, "nothing here", ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 2, items, sel)
	b.hint(c, hint)
	return b
}

func chips(c ui.Canvas, b box, y, x int, opts []string, cur string) int {
	for _, o := range opts {
		fg, bg := ui.FgSoft, ui.BgPopup
		if o == cur {
			fg, bg = ui.FgBlack, ui.FgSoft
		}
		x += b.put(c, x, y, " "+o+" ", fg, bg) + 1
	}
	return x
}

func searchRow(c ui.Canvas, b box, y int, query, count string) {
	b.fill(c, y, ui.BgTerm)
	b.put(c, 2, y, "/", ui.Blue, ui.BgTerm)
	b.put(c, 4, y, query+"▏", ui.FgWhite, ui.BgTerm)
	b.putRight(c, y, count, ui.FgDim, ui.BgTerm)
}

func searchHeader(query, count func() string) headerFn {
	return func(c ui.Canvas, b box) {
		searchRow(c, b, 0, query(), count())
	}
}
