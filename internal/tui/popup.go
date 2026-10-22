package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// box is the content area of a dialog; the painted surface (ox..) extends one
// row above and below it so every dialog has a padding of 1.
type box struct{ x, y, w, h, oy, oh int }

// modalWidth is the half-screen width, the floor for every dialog on a wide screen.
func modalWidth(sw int) int { return min(max(0, sw), max(1, sw/2)) }

// dialogWidth fits a dialog to its content on the screen it is on. On a wide
// screen it keeps the calm half-screen width (or the requested width when the
// content needs more); on a narrow one it takes nearly the whole row, because
// half of 60 columns cannot hold a key hint, and never exceeds the screen.
func dialogWidth(sw, want int) int {
	w := max(modalWidth(sw), min(want, sw-4))
	if sw < 100 {
		w = max(w, sw-4)
	}
	return min(max(w, 1), max(sw, 0))
}

func centered(sw, sh, want, h int) box {
	w := dialogWidth(sw, want)
	oh := max(0, min(h+2, sh))
	oy := (sh - oh) / 2
	return box{x: (sw - w) / 2, y: oy + 1, w: w, h: max(0, oh-2), oy: oy, oh: oh}
}

func (b box) bg(sel bool) tcell.Color {
	if sel {
		return render.BgPopupSel
	}
	return render.BgPopup
}

func (b box) fill(c render.Canvas, y int, bg tcell.Color) {
	if y < 0 || y >= b.h {
		return
	}
	inset := 0
	if bg == render.BgPopupSel || bg == render.BgPopupIn {
		inset = min(2, b.w/2)
	}
	c.Fill(b.x+inset, b.y+y, max(0, b.w-2*inset), 1, bg)
}

func (b box) put(c render.Canvas, x, y int, text string, fg, bg tcell.Color) int {
	if x < 0 || y < 0 || y >= b.h {
		return 0
	}
	return c.Text(b.x+x, b.y+y, max(0, b.w-x-2), text, tcell.StyleDefault.Foreground(fg).Background(bg))
}

func (b box) putRight(c render.Canvas, y int, text string, fg, bg tcell.Color) {
	x := max(0, b.w-2-render.TextWidth(text))
	b.put(c, x, y, text, fg, bg)
}

func (b box) hint(c render.Canvas, text string) {
	b.fill(c, b.h-1, render.BgPopup)
	b.put(c, 2, b.h-1, text, render.FgDim, render.BgPopup)
}

func (b box) frame(c render.Canvas) {
	c.Dim()
	c.Fill(b.x, b.oy, b.w, b.oh, render.BgPopup)
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

func drawItems(c render.Canvas, b box, y int, items []item, sel int) {
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
		b.put(c, 2, y, mark, render.Blue, bg)
		x := 4
		if it.glyph != "" {
			x += b.put(c, x, y, it.glyph+" ", it.glyphFG, bg)
		}
		lfg := render.FgSoft
		if it.accent {
			lfg = render.Blue
		}
		if isSel {
			lfg = render.FgWhite
		}
		labelW := max(0, b.w-x-2)
		if it.right != "" {
			labelW = max(0, labelW-render.TextWidth(it.right)-2)
		}
		if it.badge != "" {
			labelW = max(0, labelW-render.TextWidth(it.badge)-2)
			bx := b.w - 2 - render.TextWidth(it.badge)
			if it.right != "" {
				bx -= render.TextWidth(it.right) + 2
			}
			b.put(c, max(x, bx), y, it.badge, bgColor(), bg)
		}
		b.put(c, x, y, render.Truncate(it.label, labelW), lfg, bg)
		if it.right != "" {
			b.putRight(c, y, it.right, render.FgDim, bg)
		}
		y++
		if isSel && it.detail != "" && y < b.h-2 {
			b.fill(c, y, bg)
			b.put(c, 4, y, it.detail, render.FgDim, bg)
			y++
		}
	}
}

type headerFn func(c render.Canvas, b box)

func titleRow(title, sub string) headerFn {
	return func(c render.Canvas, b box) {
		titleW := max(0, b.w-4)
		if sub != "" {
			titleW = max(0, titleW-render.TextWidth(sub)-2)
		}
		b.put(c, 2, 0, render.Truncate(title, titleW), render.Blue, render.BgPopup)
		if sub != "" {
			b.putRight(c, 0, sub, render.FgDim, render.BgPopup)
		}
	}
}

func listBox(c render.Canvas, w, h, bw int, header headerFn, items []item, sel int, hint string) box {
	bh := 4 + itemsHeight(items, sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, bw, bh)
	b.frame(c)
	header(c, b)
	if len(items) == 0 {
		b.put(c, 4, 2, "nothing here", render.FgDim, render.BgPopup)
	}
	drawItems(c, b, 2, items, sel)
	b.hint(c, hint)
	return b
}

func chips(c render.Canvas, b box, y, x int, opts []string, cur string) int {
	for _, o := range opts {
		fg, bg := render.FgSoft, render.BgPopup
		if o == cur {
			fg, bg = render.FgBlack, render.FgSoft
		}
		x += b.put(c, x, y, " "+o+" ", fg, bg) + 1
	}
	return x
}

// searchRow draws the search input on a slightly darker surface. An empty
// query shows the dim placeholder after the cursor.
func searchRow(c render.Canvas, b box, y int, query *textEdit, placeholder, count string) {
	if y < 0 || y >= b.h {
		return
	}
	b.fill(c, y, render.BgPopupIn)
	b.put(c, 2, y, "/", render.Blue, render.BgPopupIn)
	queryW := max(0, b.w-6-render.TextWidth(count)-2)
	query.Placeholder = placeholder
	query.Draw(c, b.x+4, b.y+y, queryW, 1, render.Style{FG: render.FgWhite, BG: render.BgPopupIn}.TCell(), true)
	b.putRight(c, y, count, render.FgDim, render.BgPopupIn)
}

// titledSearch is the shared header of searchable lists: title row, blank row,
// search row. Lists start at row 4.
func titledSearch(c render.Canvas, b box, title, sub string, query *textEdit, placeholder string) {
	titleRow(title, sub)(c, b)
	searchRow(c, b, 2, query, placeholder, "")
}
