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
	if a.st.Delegated > 0 || a.st.DelegatedUnpriced {
		d := fmt.Sprintf("~$%.4f", a.st.Delegated)
		if a.st.DelegatedUnpriced {
			d += "+?" // some delegated requests have no known price
		}
		out += " · delegated " + d
	}
	return out
}

func (a *App) jsOpts() ui.JSOpts {
	js := ui.JSOpts{Lines: a.st.Settings.JSLines, OutLines: a.st.Settings.JSOutLines}
	js.Mode = jsMode(a.st.Settings.JSCode)
	js.OutMode = jsMode(a.st.Settings.JSOutput)
	js.ThinkMode = jsMode(a.st.Settings.Thinking)
	js.ThinkLines = a.st.Settings.ThinkLines
	switch a.st.Settings.JSTiming {
	case "slow":
		js.Timing = ui.TimingSlow
	case "off":
		js.Timing = ui.TimingOff
	}
	js.Summary = a.st.Settings.TurnSummary != "off"
	js.Images = a.imageOpts()
	js.FoldStrings = a.st.Settings.JSStrings == "lines"
	js.FoldLines = a.st.Settings.JSStringLines
	return js
}

// Composer geometry in terminal cells, derived from the design's 18/20 padding
// scaled to the cell grid (1 cell = 10px horizontally, 1 row = 18px vertically).
const (
	composerPadX = 2
	composerPadY = 1
)

// drawComposer draws the input band. Text starts at the padded origin; the real
// terminal cursor marks the edit position (see applyCursor).
func (a *App) drawComposer(c ui.Canvas, x, y, w, h int) {
	pad := min(composerPadX, max(0, (w-1)/2))
	textX := x + pad
	innerW := max(0, w-2*pad)
	row := y
	if h > composerPadY+1 {
		row = y + composerPadY
	}
	bg := ui.BgUser
	a.clampCursor()
	if len(a.st.Input) == 0 {
		a.st.ComposerRows = []vrow{{0, 0, false}}
		a.st.ComposerX, a.st.ComposerY, a.st.ComposerTop = textX, row, 0
		if !a.st.InLog {
			a.cur = cursorPos{textX, row, true}
		}
		text, fg := a.placeholder(), ui.FgDim
		c.Text(textX, row, innerW, text, ui.Style{FG: fg, BG: bg}.TCell())
		return
	}
	rows := layoutInput(a.st.Input, max(1, innerW))
	avail := max(1, h-2*composerPadY)
	if h <= composerPadY+1 {
		avail = 1
	}
	cr := cursorRow(rows, a.st.Cursor)
	// Keep the cursor row inside the visible window.
	top := min(max(0, a.st.InputTop), max(0, len(rows)-avail))
	if cr < top {
		top = cr
	}
	if cr >= top+avail {
		top = cr - avail + 1
	}
	a.st.InputTop = top
	a.st.ComposerRows, a.st.ComposerX, a.st.ComposerY, a.st.ComposerTop = rows, textX, row, top
	toks := a.inputTokens()
	mask := a.inputSelMask()
	for i := 0; i < avail && top+i < len(rows); i++ {
		a.drawInputRow(c, textX, row+i, innerW, rows[top+i], toks, bg, mask)
	}
	col := widthBetween(a.st.Input, rows[cr].start, a.st.Cursor)
	if !a.st.InLog {
		a.cur = cursorPos{textX + min(col, max(0, innerW-1)), row + cr - top, true}
	}
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
	{":agents", "working subagents"},
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
