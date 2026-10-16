package app

import (
	"strings"

	"meno/internal/ui"
)

func (a *App) modelLabel() string {
	if a.st.Settings.Model == "" {
		return "модель не выбрана · :model"
	}
	return a.st.Settings.Model + " (" + a.st.Settings.Effort + ")"
}

func (a *App) statsText() string {
	return "tokens " + itoa(a.st.Usage.In) + " / " + itoa(a.st.Usage.Out)
}

func (a *App) transcriptLines(w int) []ui.Line {
	if a.st.Session == nil {
		return nil
	}
	return ui.BuildLines(a.st.Session.Snapshot(), w, a.jsOpts(), a.st.Settings.Thinking == "on")
}

func (a *App) jsOpts() ui.JSOpts {
	js := ui.JSOpts{Output: a.st.Settings.JSOutput == "on", Lines: a.st.Settings.JSLines}
	switch a.st.Settings.JSCode {
	case "lines":
		js.Mode = ui.JSLines
	case "off":
		js.Mode = ui.JSOff
	}
	return js
}

// Composer geometry in terminal cells, derived from the design's 18/20 padding
// scaled to the cell grid (1 cell = 10px horizontally, 1 row = 18px vertically).
const (
	composerPadX = 2
	composerPadY = 1
)

// drawComposer draws the input band. Text starts at the padded origin; the cursor
// is always visible, including on an empty input.
func (a *App) drawComposer(c ui.Canvas, x, y, w, h int) {
	textX := x + composerPadX
	innerW := w - 2*composerPadX
	row := y
	if h > composerPadY+1 {
		row = y + composerPadY
	}
	bg := ui.BgUser
	if a.st.Toast != "" && len(a.st.Input) == 0 {
		c.Text(textX, row, innerW, a.st.Toast, ui.Style{FG: ui.FgDim, BG: bg}.TCell())
		return
	}
	if len(a.st.Input) == 0 {
		c.Text(textX, row, innerW, "▏", ui.Style{FG: ui.FgWhite, BG: bg}.TCell())
		c.Text(textX+2, row, innerW-2, "напиши задачу, #промпт или :команду", ui.Style{FG: ui.FgDim, BG: bg}.TCell())
		return
	}
	c.Text(textX, row, innerW, string(a.st.Input)+"▏", ui.Style{FG: ui.FgWhite, BG: bg}.TCell())
}

func (a *App) drawCommandList(c ui.Canvas, x, y, w int) {
	for i, name := range commandRows(a) {
		bg, fg := ui.BgPopup, ui.FgSoft
		mark := " "
		if i == 0 {
			bg, fg, mark = ui.BgPopupSel, ui.FgWhite, "▸"
		}
		c.Fill(x, y+i, w, 1, bg)
		c.Text(x+2, y+i, 2, mark, ui.Style{FG: ui.Blue, BG: bg}.TCell())
		c.Text(x+4, y+i, 20, name, ui.Style{FG: fg, BG: bg}.TCell())
		c.Text(x+24, y+i, w-26, cmdDesc(name), ui.Style{FG: ui.FgDim, BG: bg}.TCell())
	}
}

func cmdDesc(name string) string {
	for _, c := range commandList {
		if c.name == name {
			return c.desc
		}
	}
	return ""
}

func (a *App) drawCommandLine(c ui.Canvas, x, y, w int) {
	c.Fill(x, y, w, 1, ui.BgTerm)
	c.Text(x+2, y, w-4, ":"+string(a.st.Cmd)+"█", ui.Style{FG: ui.FgWhite, BG: ui.BgTerm}.TCell())
}

func commandRows(a *App) []string {
	var out []string
	for _, c := range commandList {
		if a.st.Cmd == nil || strings.HasPrefix(c.name, ":"+string(a.st.Cmd)) {
			out = append(out, c.name)
		}
	}
	return out
}

type cmdDef struct {
	name, desc string
}

var commandList = []cmdDef{
	{":new", "new session"},
	{":resume", "switch session"},
	{":model", "model & effort"},
	{":skills", "load skills"},
	{":mcp", "mcp servers"},
	{":prompts", "saved #prompts"},
	{":providers", "api keys & urls"},
	{":settings", "display & repl"},
	{":reset", "reset js vm"},
	{":compact", "compact context"},
	{":export", "export markdown"},
	{":stop", "stop running js"},
	{":help", "keys & commands"},
	{":q", "quit meno"},
}
