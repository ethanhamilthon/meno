package app

import (
	"strconv"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/ui"
)

type settingRow struct {
	label string
	key   string
}

var settingRows = []settingRow{
	{"js code", "js_code"},
	{"js output", "js_output"},
	{"thinking", "thinking"},
	{"js timeout", "js_timeout"},
	{"ask bell", "notify_question"},
}

func (a *App) settingsKey(k *tcell.EventKey) {
	n := len(settingRows)
	switch {
	case a.navKey(k, n, true):
		return
	case k.Key() == tcell.KeyLeft || k.Rune() == ' ' || k.Key() == tcell.KeyEnter:
		step := 1
		if k.Key() == tcell.KeyLeft {
			step = -1
		}
		a.st.Settings.Cycle(settingRows[a.st.Sel].key, step)
		a.persistSettings()
	case k.Key() == tcell.KeyRight:
		a.st.Settings.Cycle(settingRows[a.st.Sel].key, 1)
		a.persistSettings()
	case k.Rune() == '+' || k.Rune() == '=' || k.Rune() == '-':
		key := settingRows[a.st.Sel].key
		if config.NumOption(key) == "" {
			return
		}
		delta := 1
		if k.Rune() == '-' {
			delta = -1
		}
		a.st.Settings.AdjustNum(key, delta)
		a.persistSettings()
	}
}

// optLabel is how a choice of a setting is written: the numeric choices carry
// their number ("lines 5", "30 sec").
func optLabel(s *config.Settings, key, opt string) string {
	if opt != config.NumOption(key) {
		return opt
	}
	if opt == "sec" {
		return strconv.Itoa(s.NumFor(key)) + " sec"
	}
	return opt + " " + strconv.Itoa(s.NumFor(key))
}

func (a *App) drawSettings(c ui.Canvas, w, h int) {
	bh := 4 + len(settingRows) + 1
	b := centered(w, h, 66, bh)
	b.frame(c)
	b.put(c, 2, 0, "settings", ui.Blue, ui.BgPopup)
	y := 2
	for i, r := range settingRows {
		sel := i == a.st.Sel
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
		opts := config.Choices(r.key)
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
			x = max(4, b.w-2-ui.TextWidth(labels[0]))
		}
		b.put(c, 4, y, ui.Truncate(padCol(r.label, 14), max(0, x-5)), fg, bg)
		for j, o := range opts {
			label := labels[j]
			fg, obg := ui.FgDim, ui.BgPopup
			if o == cur {
				fg, obg = ui.FgBlack, ui.Blue
				if config.NumOption(r.key) == "" {
					obg = ui.FgSoft
				}
			}
			x += b.put(c, x, y, padding+label+padding, fg, obg) + 1
		}
		y++
	}
	hint := "j/k move · ←/→ change · +/- number · esc close"
	if ui.TextWidth(hint) > b.w-4 {
		hint = "j/k move · ←/→ set · esc close"
	}
	b.hint(c, hint)
}

func padCol(s string, n int) string { return ui.Pad(s, n) }
