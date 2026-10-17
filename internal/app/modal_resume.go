package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/ui"
)

func (a *App) resumeIdx() []int {
	q := string(a.st.Query)
	return filterIdx(len(a.st.Sessions), func(i int) bool {
		s := a.st.Sessions[i]
		return containsFold(s.Title, q) || containsFold(s.Cwd, q) || containsFold(s.UpdatedAt.Format("2006-01-02"), q)
	})
}

func (a *App) resumeKey(k *tcell.EventKey) {
	idx := a.resumeIdx()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyCtrlU:
		a.st.Query = nil
		a.st.Sel = 0
	case k.Key() == tcell.KeyEnter:
		if len(idx) > 0 {
			a.resumeSession(a.st.Sessions[idx[clampSel(a.st.Sel, len(idx))]].ID)
		}
	default:
		a.queryKey(k)
	}
}

func (a *App) resumeSession(id string) {
	if a.st.Running {
		a.notifyErr("stop the current turn first")
		return
	}
	if a.st.Session != nil && a.st.Session.ID == id {
		a.closeOverlay()
		return
	}
	s, err := agent.LoadSession(a.paths.SessionsDir(), id)
	if err != nil {
		a.notifyErr("resume failed: " + err.Error())
		return
	}
	a.saveSession()
	a.st.Session = s
	a.st.LastUsage = s.Metrics.LastUsage
	a.st.Usage = s.Metrics.Usage
	a.st.Spent = s.Metrics.Spent
	a.st.Context = s.Metrics.Context
	a.st.Unpriced = s.Metrics.Unpriced
	a.vm.Reset()
	a.st.Scroll = 0
	a.closeOverlay()
	a.notify("session resumed")
}

func (a *App) drawResume(c ui.Canvas, w, h int) {
	cur := ""
	if a.st.Session != nil {
		cur = a.st.Session.ID
	}
	idx := a.resumeIdx()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		s := a.st.Sessions[i]
		g, gfg := "○", ui.FgDim
		right := s.UpdatedAt.Format("2006-01-02")
		if s.ID == cur {
			g, gfg = "●", ui.Green
			right = "current · " + right
		}
		title := s.Title
		if title == "" {
			title = "untitled"
		}
		path := s.Cwd
		if path == "" {
			path = "path unavailable"
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: title, right: right,
			detail: path + " · " + itoa(s.Turns) + " turns · " + ago(s.UpdatedAt)})
	}
	bh := 6 + itemsHeight(items, a.st.Sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, 0, bh)
	b.frame(c)
	titleRow("sessions", itoa(len(items))+" / "+itoa(len(a.st.Sessions)))(c, b)
	searchRow(c, b, 2, string(a.st.Query), "search title, date or path", "")
	if len(items) == 0 {
		b.put(c, 4, 4, "no matching sessions", ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 4, items, clampSel(a.st.Sel, len(items)))
	b.hint(c, "↑/↓ select · enter resume · ctrl+u clear · esc close")
}
