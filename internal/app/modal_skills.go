package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func (a *App) skillIdx() []int {
	q := string(a.st.Query)
	return filterIdx(len(a.st.Skills), func(i int) bool {
		s := a.st.Skills[i]
		return containsFold(s.Name+" "+s.Desc, q)
	})
}

func (a *App) skillsKey(k *tcell.EventKey) {
	idx := a.skillIdx()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyRune && k.Rune() == ' ':
		if len(idx) > 0 {
			s := &a.st.Skills[idx[clampSel(a.st.Sel, len(idx))]]
			s.Enabled = !s.Enabled
			a.persistSkills()
		}
	default:
		a.queryKey(k)
	}
}

func (a *App) drawSkills(c ui.Canvas, w, h int) {
	idx := a.skillIdx()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		s := a.st.Skills[i]
		g, gfg := "○", ui.FgDim
		if s.Enabled {
			g, gfg = "●", ui.Green
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: s.Name, right: s.Scope, detail: s.Desc})
	}
	empty := ""
	if len(a.st.Skills) == 0 {
		empty = "add .md files to .meno/skills"
	}
	a.searchListEmpty(c, w, h, 73, len(a.st.Skills), "skills", "search skills", items, "↑/↓ move · space toggle · esc close", empty)
}
