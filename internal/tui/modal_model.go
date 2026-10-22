package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/render"
)

func (a *App) modelList() []llm.Model {
	return llm.FilterModels(a.allModels(), a.st.Query.String())
}

func (a *App) modelKey(k *tcell.EventKey) {
	list := a.modelList()
	switch {
	case a.navKey(k, len(list), false):
	case k.Key() == tcell.KeyLeft:
		a.cycleEffort(-1, list)
	case k.Key() == tcell.KeyRight:
		a.cycleEffort(1, list)
	case k.Key() == tcell.KeyEnter:
		if len(list) > 0 {
			pick := list[clampSel(a.st.Sel, len(list))]
			_, _, effort := a.selection()
			if a.st.EffortPick != "" {
				effort = a.st.EffortPick
			}
			// The new model may not accept the effort of the old one.
			effort = llm.ClampEffort(a.effortsFor(pick.Provider, pick.ID), effort)
			a.setSelection(pick.Provider, pick.ID, effort)
			a.persistSettings()
			a.closeOverlay()
			a.st.Mode = render.ModeInput
		}
	default:
		if a.st.Query.Handle(k) {
			a.st.Sel = 0
		}
	}
}

func effortOf(a *App) string {
	_, _, e := a.selection()
	return e
}

// effortsFor lists the effort levels of a model: its own from the settings of
// the provider, or the standard ones.
func (a *App) effortsFor(provider, model string) []string {
	standard := config.Choices("effort")
	for _, p := range a.st.Providers {
		if p.Name == provider {
			return p.EffortsFor(model, standard)
		}
	}
	return standard
}

// highlighted is the model under the cursor of the model list, or the model of
// the session when the list is empty.
func (a *App) highlighted(list []llm.Model) (provider, model string) {
	if len(list) > 0 {
		m := list[clampSel(a.st.Sel, len(list))]
		return m.Provider, m.ID
	}
	p, m, _ := a.selection()
	return p, m
}

// cycleEffort steps the candidate effort through the levels of the model under
// the cursor. Nothing is saved or sent until Enter.
func (a *App) cycleEffort(step int, list []llm.Model) {
	hp, hm := a.highlighted(list)
	levels := a.effortsFor(hp, hm)
	if len(levels) == 0 {
		return
	}
	cur := a.st.EffortPick
	if cur == "" {
		cur = effortOf(a)
	}
	idx := 0
	for i, l := range levels {
		if l == cur {
			idx = i
		}
	}
	n := len(levels)
	a.st.EffortPick = levels[((idx+step)%n+n)%n]
}

func (a *App) drawModel(c render.Canvas, w, h int) {
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
	titledSearch(c, b, "model", itoa(len(list))+" / "+itoa(len(a.allModels())), &a.st.Query, "search models")
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
		b.put(c, 2, y, mark, render.Blue, bg)
		fg := render.FgSoft
		if sel {
			fg = render.FgWhite
		}
		b.put(c, 4, y, render.Truncate(m.ID, max(0, b.w-8-render.TextWidth(m.Provider))), fg, bg)
		b.putRight(c, y, m.Provider, render.FgDim, bg)
		y++
		if sel {
			b.fill(c, y, bg)
			detail := "metadata unavailable"
			for _, provider := range a.st.Providers {
				if provider.Name == m.Provider {
					if info, ok := provider.Lookup(a.st.Pricing, m.ID); ok {
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
			b.put(c, 4, y, detail, render.FgDim, bg)
			y++
		}
	}
	if len(list) == 0 {
		b.put(c, 4, y, render.Truncate(a.emptyModelText(), max(0, b.w-8)), render.FgDim, render.BgPopup)
		y++
	}
	b.fill(c, y+1, render.BgPopup)
	b.put(c, 2, y+1, "effort", render.FgDim, render.BgPopup)
	hp, hm := a.highlighted(list)
	chosen := effortOf(a)
	if a.st.EffortPick != "" {
		chosen = a.st.EffortPick
	}
	chips(c, b, y+1, 12, a.effortsFor(hp, hm), chosen)
	b.hint(c, "↑/↓ model · ←/→ effort · enter apply · esc cancel")
}

// highlightedPair is highlighted() for the current list, for call sites that
// pass its two results on.
func (a *App) highlightedPair() (string, string) { return a.highlighted(a.modelList()) }
