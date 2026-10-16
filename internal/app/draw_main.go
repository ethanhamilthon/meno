package app

import "meno/internal/ui"

func (a *App) drawMain(c ui.Canvas, w, h int) {
	bottom := a.bottomHeight()
	logH := h - bottom
	lines := a.transcriptWithSelection(w)
	if len(lines) == 0 {
		a.drawWelcome(c, 0, 0, w, logH)
	} else {
		a.drawLog(c, 0, 0, w, logH, lines)
	}
	a.drawBottom(c, 0, logH, w, bottom)
}

func (a *App) bottomHeight() int {
	n := 0
	if a.st.Mode == ui.ModeCommand {
		n = len(commandRows(a))
	}
	return n + a.acHeight() + 2
}

func (a *App) drawLog(c ui.Canvas, x, y, w, h int, lines []ui.Line) {
	c.Fill(x, y, w, h, ui.BgTerm)
	start := 0
	if len(lines) > h {
		start = len(lines) - h - a.st.Scroll
		if start < 0 {
			start = 0
		}
	}
	for i := 0; i < h && start+i < len(lines); i++ {
		ui.DrawLine(c, x, y+i, w, lines[start+i])
	}
}

func (a *App) drawBottom(c ui.Canvas, x, y, w, h int) {
	if a.st.Mode == ui.ModeCommand {
		a.drawCommandList(c, x, y, w)
		n := len(commandRows(a))
		y += n
		h -= n
		c.Fill(x, y, w, h-1, ui.BgTerm)
		a.drawCommandLine(c, x, y, w)
		a.drawStatus(c, x, y+h-1, w)
		return
	}
	inputH := h - 1
	if ac := a.acHeight(); ac > 0 {
		a.drawAutocomplete(c, x, y, w)
		y += ac
		inputH -= ac
	}
	c.Fill(x, y, w, inputH, ui.BgUser)
	a.drawComposer(c, x, y, w, inputH)
	a.drawStatus(c, x, y+inputH, w)
}

func (a *App) drawStatus(c ui.Canvas, x, y, w int) {
	left := []ui.Seg{{Text: " " + a.st.Mode.Label() + " ", FG: ui.FgBlack}}
	midText := a.statsText()
	if h := a.visualHint(); h != "" {
		midText = h
	}
	mid := []ui.Seg{{Text: midText, FG: ui.FgDim}}
	right := []ui.Seg{
		{Text: "● ", FG: ui.Green},
		{Text: a.modelLabel(), FG: ui.FgDim},
		{Text: " ", FG: ui.FgDim},
	}
	ui.DrawBar(c, x, y, w, ui.BgTerm, left, mid, right)
	ui.Badge(c, x, y, a.st.Mode.Label(), a.st.Mode.Color())
}
