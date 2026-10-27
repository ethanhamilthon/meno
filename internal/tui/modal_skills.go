package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/render"
)

func (a *App) skillIdx() []int {
	q := a.query()
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
	case k.Key() == tcell.KeyTab:
		if len(idx) > 0 {
			s := &a.st.Skills[idx[clampSel(a.st.Sel, len(idx))]]
			s.Auto = !s.Auto
			a.persistSkills()
		}
	default:
		a.queryKey(k)
	}
}

func (a *App) drawSkills(c render.Canvas, w, h int) {
	idx := a.skillIdx()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		s := a.st.Skills[i]
		g, gfg := "○", render.FgDim
		if s.Enabled {
			g, gfg = "●", render.Green
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: s.Name, right: skillRight(s), detail: s.Desc})
	}
	empty := ""
	if len(a.st.Skills) == 0 {
		empty = "add .md files to .meno/skills"
	}
	a.searchListEmpty(c, w, h, 73, len(a.st.Skills), "skills", "search skills", items, "↑/↓ move · space on/off · tab auto-load · esc close", empty)
}

// skillRight is the scope of a skill, with "auto" when it loads into every new
// session.
func skillRight(s config.Skill) string {
	if s.Auto {
		return "auto · " + s.Scope
	}
	return s.Scope
}
