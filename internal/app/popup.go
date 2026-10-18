package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// box is the content area of a dialog; the painted surface (ox..) extends one
// row above and below it so every dialog has a padding of 1.
type box struct{ x, y, w, h, oy, oh int }

// modalWidth is shared by every dialog, regardless of its contents.
func modalWidth(sw int) int { return min(max(0, sw), max(1, sw/2)) }

func centered(sw, sh, _, h int) box {
	w := modalWidth(sw)
	oh := max(0, min(h+2, sh))
	oy := (sh - oh) / 2
	return box{x: (sw - w) / 2, y: oy + 1, w: w, h: max(0, oh-2), oy: oy, oh: oh}
}

func (b box) bg(sel bool) tcell.Color {
	if sel {
		return ui.BgPopupSel
	}
	return ui.BgPopup
}

func (b box) fill(c ui.Canvas, y int, bg tcell.Color) {
	if y < 0 || y >= b.h {
		return
	}
	inset := 0
	if bg == ui.BgPopupSel || bg == ui.BgPopupIn {
		inset = min(2, b.w/2)
	}
	c.Fill(b.x+inset, b.y+y, max(0, b.w-2*inset), 1, bg)
}

func (b box) put(c ui.Canvas, x, y int, text string, fg, bg tcell.Color) int {
	if x < 0 || y < 0 || y >= b.h {
		return 0
	}
	return c.Text(b.x+x, b.y+y, max(0, b.w-x-2), text, tcell.StyleDefault.Foreground(fg).Background(bg))
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
	c.Dim()
	c.Fill(b.x, b.oy, b.w, b.oh, ui.BgPopup)
}

type item struct {
	glyph   string
	glyphFG tcell.Color
	label   string
	accent  bool
	right   string
	badge   string // blinking purple marker left of right, e.g. "◆ 2"
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
	capacity := max(0, b.h-y-2)
	start := 0
	used := 0
	for i := 0; i <= sel && i < len(items); i++ {
		used++
		if i == sel && items[i].detail != "" {
			used++
		}
		for used > capacity && start < i {
			start++
			used--
		}
	}
	for i := start; i < len(items) && y < b.h-2; i++ {
		it := items[i]
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
		labelW := max(0, b.w-x-2)
		if it.right != "" {
			labelW = max(0, labelW-ui.TextWidth(it.right)-2)
		}
		if it.badge != "" {
			labelW = max(0, labelW-ui.TextWidth(it.badge)-2)
			bx := b.w - 2 - ui.TextWidth(it.badge)
			if it.right != "" {
				bx -= ui.TextWidth(it.right) + 2
			}
			b.put(c, max(x, bx), y, it.badge, bgColor(), bg)
		}
		b.put(c, x, y, ui.Truncate(it.label, labelW), lfg, bg)
		if it.right != "" {
			b.putRight(c, y, it.right, ui.FgDim, bg)
		}
		y++
		if isSel && it.detail != "" && y < b.h-2 {
			b.fill(c, y, bg)
			b.put(c, 4, y, it.detail, ui.FgDim, bg)
			y++
		}
	}
}

type headerFn func(c ui.Canvas, b box)

func titleRow(title, sub string) headerFn {
	return func(c ui.Canvas, b box) {
		titleW := max(0, b.w-4)
		if sub != "" {
			titleW = max(0, titleW-ui.TextWidth(sub)-2)
		}
		b.put(c, 2, 0, ui.Truncate(title, titleW), ui.Blue, ui.BgPopup)
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

// searchRow draws the search input on a slightly darker surface. An empty
// query shows the dim placeholder after the cursor.
func searchRow(c ui.Canvas, b box, y int, query, placeholder, count string) {
	if y < 0 || y >= b.h {
		return
	}
	b.fill(c, y, ui.BgPopupIn)
	b.put(c, 2, y, "/", ui.Blue, ui.BgPopupIn)
	queryW := max(0, b.w-6-ui.TextWidth(count)-2)
	if query == "" {
		x := b.x + 4
		x += c.Text(x, b.y+y, queryW, "█", ui.Style{FG: ui.FgWhite, BG: ui.BgPopupIn}.TCell())
		c.Text(x, b.y+y, max(0, queryW-1), placeholder, ui.Style{FG: ui.FgDim, BG: ui.BgPopupIn}.TCell())
	} else {
		query = ui.Truncate(query, max(0, queryW-1))
		c.Text(b.x+4, b.y+y, queryW, query+"█", ui.Style{FG: ui.FgWhite, BG: ui.BgPopupIn}.TCell())
	}
	b.putRight(c, y, count, ui.FgDim, ui.BgPopupIn)
}

// titledSearch is the shared header of searchable lists: title row, blank row,
// search row. Lists start at row 4.
func titledSearch(c ui.Canvas, b box, title, sub, query, placeholder string) {
	titleRow(title, sub)(c, b)
	searchRow(c, b, 2, query, placeholder, "")
}
