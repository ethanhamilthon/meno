package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type glyphLine struct {
	text string
	fg   tcell.Color
}

var prismLines = []glyphLine{
	{"      . · ◆ · .      ", ui.FgDim},
	{"    ╱     │     ╲    ", ui.Purple},
	{"  ╱   ◇───┼───◇   ╲  ", ui.Purple},
	{" ◆────────┼────────◆ ", ui.Blue},
	{"  ╲   ◇───┼───◇   ╱  ", ui.Purple},
	{"    ╲     │     ╱    ", ui.Purple},
	{"      ' · ◇ · '      ", ui.FgDim},
}

var wordmarkLines = []glyphLine{
	{"███▄ ▄███  ▄██████▄  ███▄   ██   ▄██████▄", ui.FgWhite},
	{"████▀████  ██▄▄▄▄██  █████▄ ██  ██      ██", ui.FgWhite},
	{"██ ▀ ▀ ██  ██▀▀▀▀▀▀  ██ ▀█████  ██      ██", ui.Blue},
	{"██     ██   ▀██████  ██   ▀███   ▀██████▀", ui.Purple},
}

type hint struct {
	key, desc string
	keyFG     tcell.Color
}

var hintsLeft = []hint{
	{":model", "выбор модели и effort", ui.Blue},
	{":skills", "подключить скиллы", ui.Blue},
	{":mcp", "серверы и тулы mcp", ui.Blue},
	{":providers", "ключи и провайдеры", ui.Blue},
}

var hintsRight = []hint{
	{"#prompt", "вставить шаблон промпта", ui.Purple},
	{":resume", "история сессий", ui.Purple},
	{"v", "visual select в буфере", ui.Purple},
	{":help", "все команды и хоткеи", ui.Purple},
}

const (
	prismW   = 20 // width of the prism column
	logoGap  = 4  // cells between prism and wordmark
	hintColW = 30
)

// drawWelcome lays out the hero block on one left edge: prism, then the wordmark
// and meta row aligned to the wordmark's left column, then the hint grid.
func (a *App) drawWelcome(c ui.Canvas, x, y, w, h int) {
	wordW := 0
	for _, wm := range wordmarkLines {
		wordW = max(wordW, ui.TextWidth(wm.text))
	}
	heroW := prismW + logoGap + wordW
	gridW := 2*hintColW + 6
	blockH := len(prismLines) + 1 + len(wordmarkLines) + 2 + len(hintsLeft)*2 + 2
	top := y + max(0, (h-blockH)/2)
	left := x + max(0, (w-max(heroW, gridW))/2)

	bg := ui.BgTerm
	for i, p := range prismLines {
		c.Text(left, top+i, w, p.text, tcell.StyleDefault.Foreground(p.fg).Background(bg))
	}
	wx := left + prismW + logoGap
	for i, wm := range wordmarkLines {
		c.Text(wx, top+1+i, w, wm.text, tcell.StyleDefault.Foreground(wm.fg).Background(bg))
	}

	metaY := top + 1 + len(wordmarkLines) + 1
	ver := "meno v0.1"
	c.Text(wx, metaY, w, ver, tcell.StyleDefault.Foreground(ui.FgSoft).Background(bg))
	dotX := wx + ui.TextWidth(ver) + 2
	c.Text(dotX, metaY, w, "·", tcell.StyleDefault.Foreground(ui.FgDim).Background(bg))
	c.Text(dotX+2, metaY, w-(dotX+2-x), a.paths.Cwd, tcell.StyleDefault.Foreground(ui.FgDim).Background(bg))

	gridY := metaY + 2
	colL := left + max(0, (heroW-gridW)/2)
	colR := colL + hintColW + 6
	for i := range hintsLeft {
		drawHint(c, colL, gridY+i*2, hintsLeft[i])
		drawHint(c, colR, gridY+i*2, hintsRight[i])
	}
}

func drawHint(c ui.Canvas, x, y int, h hint) {
	c.Text(x, y, hintColW, h.key, tcell.StyleDefault.Foreground(h.keyFG).Background(ui.BgTerm))
	c.Text(x, y+1, hintColW, h.desc, tcell.StyleDefault.Foreground(ui.FgDim).Background(ui.BgTerm))
}
