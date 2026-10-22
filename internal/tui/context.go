package tui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"meno/internal/llm"
	"meno/internal/render"
)

func usageLine(u llm.Usage) string {
	return fmt.Sprintf("in %s · out %s · cache hit %s", compactTokens(u.In), compactTokens(u.Out), cacheShare(u))
}

// usageSince is the usage added after the snapshot from.
func usageSince(now, from llm.Usage) llm.Usage {
	return llm.Usage{In: now.In - from.In, Out: now.Out - from.Out, Cached: now.Cached - from.Cached,
		CacheWrite: now.CacheWrite - from.CacheWrite, CacheReported: now.CacheReported}
}

func (a *App) contextRows() []string {
	p, m, _ := a.selection()
	spent := fmtSpend(a.st.Spent)
	if a.st.Unpriced {
		spent += " (partial)"
	}
	rows := []string{
		"model            " + p + " / " + m,
		"ctx              " + a.ctxLabel() + a.ctxPercent(),
		"spent            " + spent,
		"last turn        " + usageLine(a.st.LastUsage),
		"session          " + usageLine(a.st.Usage),
	}
	if a.st.Running {
		rows = append(rows, "current request  "+usageLine(usageSince(a.st.Usage, a.st.RunUsage0)))
	}
	return rows
}

func (a *App) ctxPercent() string {
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		return fmt.Sprintf(" (%.0f%%)", 100*float64(a.st.Context)/float64(info.Context))
	}
	return ""
}
func (a *App) onContextKey(k *tcell.EventKey) {
	if k.Key() == tcell.KeyEsc || k.Key() == tcell.KeyEnter {
		a.closeOverlay()
		return
	}
	a.navKey(k, len(a.contextRows()), false)
}
func (a *App) drawContext(c render.Canvas, w, h int) {
	rows := a.contextRows()
	b := centered(w, h, 90, min(h, len(rows)+4))
	b.frame(c)
	titleRow("context", "Esc to close")(c, b)
	visible := max(0, b.h-3)
	start := min(a.st.Sel, max(0, len(rows)-visible))
	for i := 0; i < visible && start+i < len(rows); i++ {
		b.put(c, 2, i+2, render.Truncate(rows[start+i], max(0, b.w-4)), render.FgSoft, render.BgPopup)
	}
}
