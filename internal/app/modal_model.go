package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) modelList() []llm.Model {
	return llm.FilterModels(a.allModels(), string(a.st.Query))
}

func (a *App) modelKey(k *tcell.EventKey) {
	list := a.modelList()
	switch {
	case a.navKey(k, len(list), false):
	case k.Key() == tcell.KeyLeft:
		a.st.Settings.Cycle("effort", -1)
	case k.Key() == tcell.KeyRight:
		a.st.Settings.Cycle("effort", 1)
	case k.Key() == tcell.KeyEnter:
		if len(list) > 0 {
			pick := list[clampSel(a.st.Sel, len(list))]
			a.st.Settings.Model = pick.ID
			a.st.Settings.Provider = pick.Provider
			a.persistSettings()
			a.closeOverlay()
		}
	case k.Key() == tcell.KeyBackspace || k.Key() == tcell.KeyBackspace2:
		if n := len(a.st.Query); n > 0 {
			a.st.Query = a.st.Query[:n-1]
		}
		a.st.Sel = 0
	case k.Key() == tcell.KeyRune:
		a.st.Query = append(a.st.Query, k.Rune())
		a.st.Sel = 0
	}
}

func (a *App) drawModel(c ui.Canvas, w, h int) {
	list := a.modelList()
	bw := 560
	bh := 2 + 1 + 1 + 1 + min(len(list), 8)*1 + 2 + 2
	b := centered(w, h, bw, bh)
	b.frame(c)
	searchRow(c, b, 0, string(a.st.Query), itoa(len(list))+" / "+itoa(len(a.allModels())))
	y := 2
	for i, m := range list {
		if i >= 8 {
			break
		}
		sel := i == a.st.Sel
		bg := b.bg(sel)
		b.fill(c, y, bg)
		mark := " "
		if sel {
			mark = "▸"
		}
		b.put(c, 2, y, mark, ui.Blue, bg)
		b.put(c, 4, y, m.ID, ui.FgWhite, bg)
		b.putRight(c, y, m.Provider, ui.FgDim, bg)
		y++
		if sel {
			b.fill(c, y, bg)
			b.put(c, 4, y, "provider "+m.Provider, ui.FgDim, bg)
			y++
		}
	}
	if len(list) == 0 {
		b.put(c, 4, y, "no models match", ui.FgDim, ui.BgPopup)
		y++
	}
	b.fill(c, y+1, ui.BgPopup)
	b.put(c, 2, y+1, "effort", ui.FgDim, ui.BgPopup)
	chips(c, b, y+1, 12, config.Choices("effort"), a.st.Settings.Effort)
	b.hint(c, "↑/↓ model · ←/→ effort · enter apply · esc close")
	_ = strings.TrimSpace
}
