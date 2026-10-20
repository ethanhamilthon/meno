package app

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"meno/internal/ui"
)

func (a *App) contextRows() []string {
	p, m, _ := a.selection()
	u := a.st.Usage
	cost := fmtSpend(a.st.Spent)
	if a.st.Unpriced {
		cost += " (partial; unpriced usage)"
	}
	cache := "? / ?"
	if u.CacheReported {
		cache = fmt.Sprintf("%d / %d", u.Cached, u.CacheWrite)
	}
	rows := []string{
		"Context", p + " / " + m, a.statsText(),
		"", "Session total",
		fmt.Sprintf("Input: %d   Output: %d", u.In, u.Out),
		"Cache read / write: " + cache,
		"Cost: " + cost,
		"", "Cache is read/write. Cache reads are part of input, not extra input.",
		"Context size is the latest request estimate, not lifetime token usage.",
	}
	if a.st.Session != nil {
		turns := a.st.Session.Turns()
		for i := len(turns) - 1; i >= 0; i-- {
			if turns[i].Run != nil {
				r := turns[i].Run
				rows = append(rows, "", "Latest finalized run")
				for _, line := range ui.SummaryLines(r, 80) {
					rows = append(rows, line.Text())
				}
				break
			}
		}
	}
	if a.st.Running {
		rows = append(rows, "", "Current run (provisional)", fmt.Sprintf("Input: %d   Output: %d", a.st.Usage.In-a.st.RunUsage0.In, a.st.Usage.Out-a.st.RunUsage0.Out))
	}
	return rows
}
func (a *App) onContextKey(k *tcell.EventKey) {
	if k.Key() == tcell.KeyEsc || k.Key() == tcell.KeyEnter {
		a.closeOverlay()
		return
	}
	a.navKey(k, len(a.contextRows()), false)
}
func (a *App) drawContext(c ui.Canvas, w, h int) {
	rows := a.contextRows()
	b := centered(w, h, 90, min(h, len(rows)+4))
	b.frame(c)
	titleRow("context", "Esc to close")(c, b)
	visible := max(0, b.h-3)
	start := min(a.st.Sel, max(0, len(rows)-visible))
	for i := 0; i < visible && start+i < len(rows); i++ {
		b.put(c, 2, i+2, ui.Truncate(rows[start+i], max(0, b.w-4)), ui.FgSoft, ui.BgPopup)
	}
}
