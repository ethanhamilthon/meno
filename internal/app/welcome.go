package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

var prismLines = []struct {
	text string
	fg   tcell.Color
}{
	{"      . · ◆ · .      ", ui.FgDim},
	{"    ╱     │     ╲    ", ui.Purple},
	{"  ╱   ◇───┼───◇   ╲  ", ui.Purple},
	{" ◆────────┼────────◆ ", ui.Blue},
	{"  ╲   ◇───┼───◇   ╱  ", ui.Purple},
	{"    ╲     │     ╱    ", ui.Purple},
	{"      ' · ◇ · '      ", ui.FgDim},
}

var wordmarkLines = []struct {
	text string
	fg   tcell.Color
}{
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

func (a *App) drawWelcome(c ui.Canvas, x, y, w, h int) {
	const heroW = 46
	const logoGap = 4
	block := len(prismLines)
	total := block + 1 + len(wordmarkLines) + 2 + 2 + len(hintsLeft) + 2
	top := y + max(0, (h-total)/2)
	px := x + max(0, (w-heroW)/2)
	for i, p := range prismLines {
		c.Text(px, top+i, w, p.text, tcell.StyleDefault.Foreground(p.fg).Background(ui.BgTerm))
	}
	wx := px + 20 + logoGap
	ty := top + 1
	for i, wm := range wordmarkLines {
		c.Text(wx, ty+i, w, wm.text, tcell.StyleDefault.Foreground(wm.fg).Background(ui.BgTerm))
	}
	meta := top + block + 1
	c.Text(px+20+logoGap, meta, w, "meno v0.1", tcell.StyleDefault.Foreground(ui.FgSoft).Background(ui.BgTerm))
	c.Text(px+20+logoGap+11, meta, w, "·", tcell.StyleDefault.Foreground(ui.FgDim).Background(ui.BgTerm))
	c.Text(px+20+logoGap+13, meta, w, a.paths.Cwd, tcell.StyleDefault.Foreground(ui.FgDim).Background(ui.BgTerm))

	grid := meta + 3
	colL := x + max(0, (w-60)/2)
	colR := colL + 30
	for i := range hintsLeft {
		drawHint(c, colL, grid+i*2, hintsLeft[i])
		drawHint(c, colR, grid+i*2, hintsRight[i])
	}
}

func drawHint(c ui.Canvas, x, y int, h hint) {
	c.Text(x, y, 30, h.key, tcell.StyleDefault.Foreground(h.keyFG).Background(ui.BgTerm))
	c.Text(x, y+1, 30, h.desc, tcell.StyleDefault.Foreground(ui.FgDim).Background(ui.BgTerm))
}
