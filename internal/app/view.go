package app

import (
	"strings"

	"meno/internal/ui"
)

func (a *App) modelLabel() string {
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

func (a *App) drawComposer(c ui.Canvas, x, y, w int) {
	if a.st.Mode == ui.ModeCommand {
		return
	}
	text := string(a.st.Input)
	if a.st.Toast != "" && text == "" {
		c.Text(x+2, y, w-4, a.st.Toast, ui.Style{FG: ui.FgDim, BG: ui.BgUser}.TCell())
		return
	}
	if text == "" {
		c.Text(x+2, y, w-4, "напиши задачу, #промпт или :команду", ui.Style{FG: ui.FgDim, BG: ui.BgUser}.TCell())
		return
	}
	c.Text(x+2, y, w-4, text+"▏", ui.Style{FG: ui.FgWhite, BG: ui.BgUser}.TCell())
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
