package app

import (
	"strconv"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/ui"
)

// optLabel is how a choice of a setting is written: the numeric choices carry
// their number ("lines 5", "30 sec").
func optLabel(s *config.Settings, key, opt string) string {
	switch {
	case key == "js_timing" && opt == "slow":
		return "slow only"
	case key == "compact_percent":
		return "at " + strconv.Itoa(s.CompactPercent) + "%"
	case key == "notify_volume":
		return opt + "%"
	case (key == "subagent_provider" || key == "subagent_model" || key == "subagent_effort") && opt == "":
		return "inherit"
	case opt != config.NumOption(key):
		return opt
	case opt == "sec":
		return strconv.Itoa(s.NumFor(key)) + " sec"
	default:
		return opt + " " + strconv.Itoa(s.NumFor(key))
	}
}

func (a *App) drawSettings(c ui.Canvas, w, h int) {
	tabIdx, _, rowInTab, tab := settingTabFor(a.st.Sel)
	bh := 4 + len(tab.rows) + 1
	b := centered(w, h, 66, bh)
	b.frame(c)
	b.put(c, 2, 0, "settings", ui.Blue, ui.BgPopup)
	tx := 13
	for i, t := range settingTabs {
		need := ui.TextWidth(t.name) + 2
		if tx+need > b.w-2 {
			break
		}
		if i == tabIdx {
			b.put(c, tx, 0, " "+t.name+" ", ui.FgBlack, ui.Blue)
		} else {
			b.put(c, tx, 0, " "+t.name+" ", ui.FgDim, ui.BgPopup)
		}
		tx += need + 1
	}
	y := 2
	visible := max(1, b.h-5)
	start := max(0, rowInTab-visible+1)
	for i := start; i < min(len(tab.rows), start+visible); i++ {
		r := tab.rows[i]
		sel := i == rowInTab
		bg := ui.BgPopup
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
		cur := a.st.Settings.Get(r.key)
		if config.TextSetting(r.key) {
			b.put(c, 4, y, ui.Truncate(r.label, max(1, b.w-10)), fg, bg)
			text := "(not set)"
			if a.st.Editing && sel {
				lw := ui.TextWidth(ui.Truncate(r.label, max(1, b.w-10)))
				a.st.Edit.Draw(c, b.x+4+lw+2, b.y+y, max(1, b.w-lw-8), 1, ui.Style{FG: ui.FgWhite, BG: bg}.TCell(), true)
				y++
				continue
			} else if cur != "" {
				text = llm.Mask(cur)
			}
			b.putRight(c, y, text, ui.FgDim, bg)
			y++
			continue
		}
		opts := a.settingChoices(r.key)
		labels := make([]string, len(opts))
		width := 0
		for j, o := range opts {
			labels[j] = optLabel(&a.st.Settings, r.key, o)
			width += ui.TextWidth(labels[j]) + 3
		}
		x, padding := 20, " "
		if width-1 > b.w-x-2 {
			x, padding = 16, ""
			width -= 2 * len(opts)
		}
		if width-1 > b.w-x-2 {
			for j, o := range opts {
				if o == cur {
					opts, labels = []string{o}, []string{labels[j]}
					break
				}
			}
			if len(labels) > 0 {
				x = max(4, b.w-2-ui.TextWidth(labels[0]))
			}
		}
		b.put(c, 4, y, ui.Truncate(r.label, max(1, x-5)), fg, bg)
		for j, o := range opts {
			text := padding + labels[j] + padding
			if o == cur {
				cbg := ui.FgDim
				if sel {
					cbg = ui.Blue
				}
				b.put(c, x, y, text, ui.FgBlack, cbg)
			} else {
				b.put(c, x, y, text, ui.FgDim, bg)
			}
			x += ui.TextWidth(text) + 1
		}
		y++
	}
	hint := "tab switch · ↑/↓ select · ←/→ change · +/- lines/sec · esc close"
	if a.st.Editing {
		hint = "type the key · enter save · esc cancel"
	} else if config.TextSetting(settingRows[clampSel(a.st.Sel, len(settingRows))].key) {
		hint = "enter edit the value · tab switch · esc close"
	}
	b.hint(c, hint)
}
