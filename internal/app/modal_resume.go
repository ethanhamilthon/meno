package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/ui"
)

func (a *App) resumeIdx() []int {
	q := string(a.st.Query)
	return filterIdx(len(a.st.Sessions), func(i int) bool {
		return containsFold(a.st.Sessions[i].Title, q)
	})
}

func (a *App) resumeKey(k *tcell.EventKey) {
	idx := a.resumeIdx()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyEnter:
		if len(idx) > 0 {
			a.resumeSession(a.st.Sessions[idx[clampSel(a.st.Sel, len(idx))]].ID)
		}
	default:
		a.queryKey(k)
	}
}

func (a *App) resumeSession(id string) {
	if a.st.Session != nil && a.st.Session.ID == id {
		a.closeOverlay()
		return
	}
	s, err := agent.LoadSession(a.paths.SessionsDir(), id)
	if err != nil {
		a.st.Toast = "resume failed: " + err.Error()
		return
	}
	a.saveSession()
	a.st.Session = s
	a.st.Scroll = 0
	a.closeOverlay()
	a.st.Toast = "session resumed"
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
		right := ago(s.UpdatedAt)
		if s.ID == cur {
			g, gfg = "●", ui.Green
			right = "current · " + right
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: s.Title, right: right,
			detail: itoa(s.Turns) + " turns · " + itoa(s.JSRuns) + " js runs"})
	}
	a.searchList(c, w, h, 660, len(a.st.Sessions), items, "↑/↓ select · enter resume · esc close")
}
