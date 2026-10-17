package app

import (
	"fmt"

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
		a.cycleEffort(-1)
	case k.Key() == tcell.KeyRight:
		a.cycleEffort(1)
	case k.Key() == tcell.KeyEnter:
		if len(list) > 0 {
			pick := list[clampSel(a.st.Sel, len(list))]
			_, _, effort := a.selection()
			a.setSelection(pick.Provider, pick.ID, effort)
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

func effortOf(a *App) string {
	_, _, e := a.selection()
	return e
}

// cycleEffort steps the effort of the current session through the choices.
func (a *App) cycleEffort(step int) {
	p, m, e := a.selection()
	s := config.Settings{Effort: e}
	s.Cycle("effort", step)
	a.setSelection(p, m, s.Effort)
}

func (a *App) drawModel(c ui.Canvas, w, h int) {
	list := a.modelList()
	bw := 66
	shown := min(len(list), 8)
	body := shown
	if len(list) > 0 {
		body++ // detail line of the selected model
	} else {
		body = 1
	}
	bh := 4 + body + 3 // title, gap, search, gap | gap, effort, gap+hint
	b := centered(w, h, bw, bh)
	b.frame(c)
	titledSearch(c, b, "model", itoa(len(list))+" / "+itoa(len(a.allModels())), string(a.st.Query), "search models")
	y := 4
	rows := max(0, min(8, b.h-8)) // box height minus title, search, effort, hint and the detail line
	start := max(0, a.st.Sel-rows+1)
	for i := start; i < len(list) && i < start+rows; i++ {
		m := list[i]
		sel := i == a.st.Sel
		bg := b.bg(sel)
		b.fill(c, y, bg)
		mark := " "
		if sel {
			mark = "▸"
		}
		b.put(c, 2, y, mark, ui.Blue, bg)
		fg := ui.FgSoft
		if sel {
			fg = ui.FgWhite
		}
		b.put(c, 4, y, ui.Truncate(m.ID, max(0, b.w-8-ui.TextWidth(m.Provider))), fg, bg)
		b.putRight(c, y, m.Provider, ui.FgDim, bg)
		y++
		if sel {
			b.fill(c, y, bg)
			detail := "metadata unavailable"
			for _, provider := range a.st.Providers {
				if provider.Name == m.Provider {
					if info, ok := llm.LookupMetadata(a.st.Pricing, provider, m.ID); ok {
						context := "?"
						if info.Context > 0 {
							context = compactTokens(info.Context)
						}
						detail = context + " ctx · pricing unavailable"
						if !info.MissingPricing {
							detail = fmt.Sprintf("%s ctx · $%g / $%g per 1M", context, info.Input, info.Output)
						}
					}
					break
				}
			}
			b.put(c, 4, y, detail, ui.FgDim, bg)
			y++
		}
	}
	if len(list) == 0 {
		b.put(c, 4, y, "no models match", ui.FgDim, ui.BgPopup)
		y++
	}
	b.fill(c, y+1, ui.BgPopup)
	b.put(c, 2, y+1, "effort", ui.FgDim, ui.BgPopup)
	chips(c, b, y+1, 12, config.Choices("effort"), effortOf(a))
	b.hint(c, "↑/↓ model · ←/→ effort · enter apply · esc close")
}
