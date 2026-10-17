package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type glyphLine struct {
	text string
	fg   tcell.Color
}

func prismLines() []glyphLine {
	return []glyphLine{
		{"      . · ◆ · .      ", ui.FgDim},
		{"    ╱     │     ╲    ", ui.Purple},
		{"  ╱   ◇───┼───◇   ╲  ", ui.Purple},
		{" ◆────────┼────────◆ ", ui.Blue},
		{"  ╲   ◇───┼───◇   ╱  ", ui.Purple},
		{"    ╲     │     ╱    ", ui.Purple},
		{"      ' · ◇ · '      ", ui.FgDim},
	}
}

func wordmarkLines() []glyphLine {
	return []glyphLine{
		{"███▄ ▄███  ▄██████▄  ███▄   ██   ▄██████▄", ui.Blue},
		{"████▀████  ██▄▄▄▄██  █████▄ ██  ██      ██", ui.Blue},
		{"██ ▀ ▀ ██  ██▀▀▀▀▀▀  ██ ▀█████  ██      ██", ui.Blue},
		{"██     ██   ▀██████  ██   ▀███   ▀██████▀", ui.Blue},
	}
}

type hint struct {
	key, desc string
	keyFG     tcell.Color
}

func hintsLeft() []hint {
	return []hint{
		{":model", "choose model and effort", ui.Blue},
		{":skills", "enable skills", ui.Blue},
		{":mcp", "mcp servers and tools", ui.Blue},
		{":providers", "keys and providers", ui.Blue},
	}
}

func hintsRight() []hint {
	return []hint{
		{"/skill", "use an enabled skill", ui.Purple},
		{":resume", "session history", ui.Purple},
		{"v", "visual select in buffer", ui.Purple},
		{":help", "all commands and shortcuts", ui.Purple},
	}
}

const (
	prismW   = 20 // width of the prism column
	logoGap  = 4  // cells between prism and wordmark
	hintColW = 30
)

// drawWelcome centers the hero and hint grid independently, as in the reference.
func (a *App) drawWelcome(c ui.Canvas, x, y, w, h int) {
	wordW := 0
	for _, wm := range wordmarkLines() {
		wordW = max(wordW, ui.TextWidth(wm.text))
	}
	heroW := prismW + logoGap + wordW
	gridW := 74
	compact := w < heroW+4
	blockH := 16
	if compact {
		blockH = 14
	}
	top := y + max(0, (h-blockH)/2)
	bg := ui.BgTerm
	wx := x + max(0, (w-heroW)/2) + prismW + logoGap
	if compact {
		wx = x + max(0, (w-4)/2)
		c.Text(wx, top+1, w-(wx-x), "meno", ui.Style{FG: ui.FgWhite, BG: bg, Bold: true}.TCell())
	} else {
		left := x + (w-heroW)/2
		for i, p := range prismLines() {
			if top+i < y+h {
				c.Text(left, top+i, prismW+1, p.text, ui.Style{FG: p.fg, BG: bg}.TCell())
			}
		}
		for i, wm := range wordmarkLines() {
			if top+1+i < y+h {
				c.Text(wx, top+1+i, w-(wx-x), wm.text, ui.Style{FG: wm.fg, BG: bg}.TCell())
			}
		}
	}
	metaY := top + 6
	if compact {
		metaY = top + 3
		wx = x + max(0, (w-9)/2)
	}
	if metaY < y+h {
		c.Text(wx, metaY, w-(wx-x), "meno v0.1", ui.Style{FG: ui.FgSoft, BG: bg}.TCell())
		if !compact {
			c.Text(wx+11, metaY, max(0, w-(wx+11-x)), "· "+a.paths.Cwd, ui.Style{FG: ui.FgDim, BG: bg}.TCell())
		}
	}
	gridY := top + 9
	if compact {
		gridY = top + 5
	}
	colL := x + max(0, (w-gridW)/2)
	hl, hr := hintsLeft(), hintsRight()
	for i := range hl {
		row := gridY + i*2
		if row >= y+h {
			break
		}
		if w >= gridW {
			drawHint(c, colL, row, hl[i])
			drawHint(c, colL+40, row, hr[i])
		} else {
			c.Text(x+2, row, max(0, w-4), hl[i].key+"  "+hl[i].desc, ui.Style{FG: ui.Blue, BG: bg}.TCell())
			if row+1 < y+h {
				c.Text(x+2, row+1, max(0, w-4), hr[i].key+"  "+hr[i].desc, ui.Style{FG: ui.Purple, BG: bg}.TCell())
			}
		}
	}

}

func drawHint(c ui.Canvas, x, y int, h hint) {
	c.Text(x, y, hintColW, h.key, tcell.StyleDefault.Foreground(h.keyFG).Background(ui.BgTerm))
	c.Text(x+12, y, hintColW-8, h.desc, tcell.StyleDefault.Foreground(ui.FgDim).Background(ui.BgTerm))
}
