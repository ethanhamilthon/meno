package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) modelLabel() string {
	_, model, effort := a.selection()
	if model == "" {
		return "no model selected · :model"
	}
	if effort == "" {
		return model
	}
	return model + " (" + effort + ")"
}

func (a *App) statsText() string {
	u := a.st.LastUsage
	cost := "n/a"
	info, ok := a.modelInfo()
	if a.st.Unpriced {
		cost = "n/a"
	} else if ok || a.st.Spent > 0 {
		cost = fmt.Sprintf("~$%.4f", a.st.Spent)
	}
	ctx := itoa(a.st.Context)
	if ok && info.Context > 0 {
		ctx += fmt.Sprintf("/%d %.0f%%", info.Context, 100*float64(a.st.Context)/float64(info.Context))
	}
	out := fmt.Sprintf("last in %d · out %d · cache %s · session in %d · out %d · cache %s · spent %s · ctx %s",
		u.In, u.Out, cacheText(u), a.st.Usage.In, a.st.Usage.Out, cacheText(a.st.Usage), cost, ctx)
	return out
}

func (a *App) jsOpts() ui.JSOpts {
	js := ui.JSOpts{Lines: a.st.Settings.BashLines, OutLines: a.st.Settings.BashOutputLines}
	js.Mode = jsMode(a.st.Settings.BashCode)
	js.OutMode = jsMode(a.st.Settings.BashOutput)
	js.ThinkMode = jsMode(a.st.Settings.Thinking)
	js.ThinkLines = a.st.Settings.ThinkLines
	switch a.st.Settings.BashTiming {
	case "slow":
		js.Timing = ui.TimingSlow
	case "off":
		js.Timing = ui.TimingOff
	}
	js.Summary = a.st.Settings.TurnSummary != "off"
	js.Images = a.imageOpts()
	js.FoldStrings = a.st.Settings.BashStrings == "lines"
	js.FoldLines = a.st.Settings.BashStringLines
	return js
}

func (a *App) drawCommandRows(c ui.Canvas, x, y, w, rows int) {
	all := commandRows(a)
	_, cd := a.cdRows()
	cells := make([]acCell, len(all))
	for i, name := range all {
		if cd {
			// ":cd /a/b/" shows as "b/" with the parent as the description.
			p := strings.TrimSuffix(strings.TrimPrefix(name, ":cd "), "/")
			cells[i] = acCell{filepath.Base(p) + "/", filepath.Dir(p), ui.FgWhite}
			continue
		}
		cells[i] = acCell{name, cmdDesc(name), ui.FgWhite}
	}
	drawAcGrid(c, x, y, w, rows, cells, a.st.CmdSel)
}

func cmdDesc(name string) string {
	for _, c := range commandList {
		if c.name == name {
			return c.desc
		}
	}
	return ""
}

// drawCommandLine is the whole bottom input in command mode: no padding, no status.
func (a *App) drawCommandLine(c ui.Canvas, x, y, w int) {
	c.Fill(x, y, w, 1, ui.BgUser)
	rs := []rune(":" + string(a.st.Cmd))
	for len(rs) > 0 && ui.TextWidth(string(rs))+1 > w {
		rs = rs[1:]
	}
	text := string(rs)
	c.Text(x, y, w, text, ui.Style{FG: ui.FgWhite, BG: ui.BgUser}.TCell())
	a.cur = cursorPos{x + min(ui.TextWidth(text), max(0, w-1)), y, true}
}

func commandRows(a *App) []string {
	if rows, ok := a.cdRows(); ok {
		return rows
	}
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
	{":fork", "fork at a message"},
	{":rename", "rename this session"},
	{":tasks", "background processes"},
	{":sessions", "switch session"},
	{":cd", "change directory"},
	{":model", "model & effort"},
	{":skills", "load skills"},
	{":mcp", "mcp servers"},
	{":login", "accounts & api keys"},
	{":settings", "settings"},
	{":compact", "compact context"},
	{":snippets", "saved snippets"},
	{":save_snippet", "save the selection"},
	{":paste_snippet", "insert a snippet"},
	{":export", "export markdown"},
	{":stop", "stop this turn"},
	{":help", "keys & commands"},
	{":q", "quit meno"},
	{":q!", "quit, stopping all turns"},
}

func jsMode(v string) ui.JSMode {
	switch v {
	case "lines":
		return ui.JSLines
	case "off":
		return ui.JSOff
	}
	return ui.JSFull
}

// cacheText says what the cache did for one request or for a whole session:
// tokens read from it, written to it, and the read share of the input. A
// provider that reports nothing about its cache is "unknown", never 0%.
func cacheText(u llm.Usage) string {
	if !u.CacheReported {
		return "unknown"
	}
	share := 0.0
	if u.In > 0 {
		share = 100 * float64(min(max(0, u.Cached), u.In)) / float64(u.In)
	}
	return fmt.Sprintf("read %d (%.0f%% of input) · write %d", u.Cached, share, u.CacheWrite)
}
