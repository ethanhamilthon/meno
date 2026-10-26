package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"meno/internal/llm"
	"meno/internal/render"
)

func (a *App) modelLabel() string {
	_, model, effort := a.selection()
	if model == "" {
		return "no model selected · /model"
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

func (a *App) jsOpts() render.JSOpts {
	js := render.JSOpts{View: viewMode(a.st.Settings.View)}
	js.Summary = a.st.Settings.TurnSummary != "off"
	return js
}

func (a *App) drawCommandRows(c render.Canvas, x, y, w, rows int) {
	all := commandRows(a)
	_, cd := a.cdRows()
	cells := make([]acCell, len(all))
	for i, name := range all {
		if cd {
			// "/cd /a/b/" shows as "b/" with the parent as the description.
			p := strings.TrimSuffix(strings.TrimPrefix(name, "/cd "), "/")
			cells[i] = acCell{filepath.Base(p) + "/", filepath.Dir(p), render.FgWhite}
			continue
		}
		if sk, ok := skillRowName(name); ok {
			cells[i] = acCell{name, a.skillDesc(sk), render.Blue}
			continue
		}
		cells[i] = acCell{name, cmdDesc(name), render.FgWhite}
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

func commandRows(a *App) []string {
	if rows, ok := a.cdRows(); ok {
		return rows
	}
	var out []string
	for _, c := range commandList {
		if strings.HasPrefix(c.name, "/"+a.cmdQuery()) {
			out = append(out, c.name)
		}
	}
	return append(out, a.skillRows(a.cmdQuery())...)
}

type cmdDef struct {
	name, desc string
}

var commandList = []cmdDef{
	{"/context", "context, tokens, cache and cost"},
	{"/new", "new session"},
	{"/fork", "fork at a message"},
	{"/rename", "rename this session"},
	{"/tasks", "background processes"},
	{"/sessions", "switch session"},
	{"/cd", "change directory"},
	{"/model", "model & effort"},
	{"/skills", "load skills"},
	{"/skill:", "insert a skill reference"},
	{"/mcp", "mcp servers"},
	{"/login", "accounts & api keys"},
	{"/settings", "settings"},
	{"/compact", "compact context"},
	{"/export", "export markdown"},
	{"/stop", "stop this turn"},
	{"/help", "keys & commands"},
	{"/q", "leave meno; work keeps running"},
}

func viewMode(v string) render.ViewMode {
	switch v {
	case "full":
		return render.ViewFull
	case "bash_only":
		return render.ViewBashOnly
	}
	return render.ViewCompact
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
