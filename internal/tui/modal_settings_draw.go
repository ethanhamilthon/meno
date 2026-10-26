package tui

import (
	"strconv"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/render"
)

// optLabel is how a choice of a setting is written: the numeric choices carry
// their number ("lines 5", "30 sec").
func optLabel(s *config.Settings, key, opt string) string {
	switch {
	case key == "view" && opt == "bash_only":
		return "bash only"
	case key == "compact_percent":
		return "at " + strconv.Itoa(s.CompactPercent) + "%"
	case key == "notify_volume":
		return opt + "%"
	case opt != config.NumOption(key):
		return opt
	default:
		return opt + " " + strconv.Itoa(s.NumFor(key))
	}
}

func (a *App) drawSettings(c render.Canvas, w, h int) {
	bh := 5 + len(settingRows)
	b := a.panelBox(w, h, 66, bh)
	b.frame(c)
	b.put(c, 2, 0, "settings", render.Blue, render.BgTerm)
	rowInTab := clampSel(a.st.Sel, len(settingRows))
	y := 2
	visible := max(1, b.h-5)
	start := max(0, rowInTab-visible+1)
	for i := start; i < min(len(settingRows), start+visible); i++ {
		r := settingRows[i]
		sel := i == rowInTab
		bg := b.bg(sel)
		a.panelHits[b.y+y] = i
		b.fill(c, y, bg)
		mark, labelFG, valueFG := " ", render.FgSoft, render.FgDim
		if sel {
			mark, labelFG, valueFG = "▸", render.Blue, render.FgWhite
		}
		b.put(c, 2, y, mark, render.Blue, bg)
		value := optLabel(&a.st.Settings, r.key, a.st.Settings.Get(r.key))
		if config.TextSetting(r.key) {
			if value == "" {
				value = "(not set)"
			} else {
				value = llm.Mask(value)
			}
		}
		value = render.Truncate(value, max(1, b.w/2-2))
		labelWidth := max(1, b.w-8-render.TextWidth(value))
		if a.st.Editing && sel {
			labelWidth = max(1, b.w/2-6)
		}
		b.put(c, 4, y, render.Truncate(r.label, labelWidth), labelFG, bg)
		if a.st.Editing && sel {
			fieldX := max(6, b.w/2)
			a.st.Edit.Draw(c, b.x+fieldX, b.y+y, max(1, b.w-fieldX-2), 1, render.Style{FG: render.FgWhite, BG: bg}.TCell(), true)
		} else {
			b.putRight(c, y, value, valueFG, bg)
		}
		y++
	}
	scope := "s scope: shared"
	if a.st.LocalScope {
		scope = "s scope: this window"
	}
	hint := "↑/↓ select · ←/→ change · " + scope + " · r reset · esc close"
	if a.st.Editing {
		hint = "type the key · enter save · esc cancel"
	} else if config.TextSetting(settingRows[clampSel(a.st.Sel, len(settingRows))].key) {
		hint = "enter edit the value · esc close"
	}
	b.hint(c, hint)
}
