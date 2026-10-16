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
	{"permissions", "permissions"},
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
	case k.Rune() == '+' || k.Rune() == '=':
		a.st.Settings.AdjustLines(1)
		a.persistSettings()
	case k.Rune() == '-':
		a.st.Settings.AdjustLines(-1)
		a.persistSettings()
	}
}

func (a *App) drawSettings(c ui.Canvas, w, h int) {
	bh := 4 + len(settingRows) + 1
	b := centered(w, h, 560, bh)
	b.frame(c)
	b.put(c, 2, 0, "settings", ui.Blue, ui.BgPopup)
	y := 2
	for i, r := range settingRows {
		sel := i == a.st.Sel
		bg := b.bg(sel)
		b.fill(c, y, bg)
		mark := " "
		if sel {
			mark = "▸"
		}
		b.put(c, 2, y, mark, ui.Blue, bg)
		b.put(c, 4, y, padCol(r.label, 14), ui.FgSoft, bg)
		x := 20
		cur := a.st.Settings.Get(r.key)
		for _, o := range config.Choices(r.key) {
			label := o
			if r.key == "js_code" && o == "lines" {
				label = "lines " + strconv.Itoa(a.st.Settings.JSLines)
			}
			fg, obg := ui.FgSoft, ui.BgPopup
			if o == cur {
				fg, obg = ui.FgBlack, ui.Blue
				if r.key != "js_code" {
					obg = ui.FgSoft
				}
			}
			x += b.put(c, x, y, " "+label+" ", fg, obg) + 1
		}
		y++
	}
	b.hint(c, "j/k move · ←/→ change · +/- lines · esc close")
}

func padCol(s string, n int) string { return ui.Pad(s, n) }
