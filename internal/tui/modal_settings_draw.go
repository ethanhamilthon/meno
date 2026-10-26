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
	tabIdx, _, rowInTab, tab := settingTabFor(a.st.Sel)
	bh := 4 + len(tab.rows) + 1
	b := centered(w, h, 66, bh)
	b.frame(c)
	b.put(c, 2, 0, "settings", render.Blue, render.BgPopup)
	tx := 13
	for i, t := range settingTabs {
		need := render.TextWidth(t.name) + 2
		if tx+need > b.w-2 {
			break
		}
		if i == tabIdx {
			b.put(c, tx, 0, " "+t.name+" ", render.FgBlack, render.Blue)
		} else {
			b.put(c, tx, 0, " "+t.name+" ", render.FgDim, render.BgPopup)
		}
		tx += need + 1
	}
	y := 2
	visible := max(1, b.h-5)
	start := max(0, rowInTab-visible+1)
	for i := start; i < min(len(tab.rows), start+visible); i++ {
		r := tab.rows[i]
		sel := i == rowInTab
		bg := render.BgPopup
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
		cur := a.st.Settings.Get(r.key)
		if config.TextSetting(r.key) {
			b.put(c, 4, y, render.Truncate(r.label, max(1, b.w-10)), fg, bg)
			text := "(not set)"
			if a.st.Editing && sel {
				lw := render.TextWidth(render.Truncate(r.label, max(1, b.w-10)))
				a.st.Edit.Draw(c, b.x+4+lw+2, b.y+y, max(1, b.w-lw-8), 1, render.Style{FG: render.FgWhite, BG: bg}.TCell(), true)
				y++
				continue
			} else if cur != "" {
				if r.key == "notify_command" {
					text = llm.Mask(cur)
				} else {
					text = cur
				}
			}
			b.putRight(c, y, text, render.FgDim, bg)
			y++
			continue
		}
		opts := a.settingChoices(r.key)
		labels := make([]string, len(opts))
		width := 0
		for j, o := range opts {
			labels[j] = optLabel(&a.st.Settings, r.key, o)
			width += render.TextWidth(labels[j]) + 3
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
				x = max(4, b.w-2-render.TextWidth(labels[0]))
			}
		}
		b.put(c, 4, y, render.Truncate(r.label, max(1, x-5)), fg, bg)
		for j, o := range opts {
			text := padding + labels[j] + padding
			if o == cur {
				cbg := render.FgDim
				if sel {
					cbg = render.Blue
				}
				b.put(c, x, y, text, render.FgBlack, cbg)
			} else {
				b.put(c, x, y, text, render.FgDim, bg)
			}
			x += render.TextWidth(text) + 1
		}
		y++
	}
	scope := "s scope: shared"
	if a.st.LocalScope {
		scope = "s scope: this window"
	}
	hint := "tab switch · ↑/↓ select · ←/→ change · " + scope + " · r reset · esc close"
	if a.st.Editing {
		hint = "type the key · enter save · esc cancel"
	} else if config.TextSetting(settingRows[clampSel(a.st.Sel, len(settingRows))].key) {
		hint = "enter edit the value · tab switch · esc close"
	}
	b.hint(c, hint)
}
