package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type helpRow struct{ key, desc string }

var helpRows = []helpRow{
	{"", "modes & navigation"},
	{"i / a", "insert mode (input)"},
	{"esc", "normal mode (scroll)"},
	{"v / V", "visual select in log"},
	{"y / p", "yank / paste to input"},
	{"r", "run selected in js vm"},
	{"#", "insert saved prompt"},
	{"ctrl+c", "abort running js/turn"},
	{"", ""},
	{"", "direct commands"},
	{":new", "start clean session"},
	{":reset", "restart quickjs vm"},
	{":compact", "summarize old context"},
	{":export", "save log to markdown"},
	{":clear", "clear viewport log"},
	{":stop", "kill active js task"},
	{":q", "exit meno"},
}

func (a *App) helpKey(k *tcell.EventKey) {
	switch {
	case k.Rune() == 'q' || k.Key() == tcell.KeyEnter:
		a.closeOverlay()
	case k.Key() == tcell.KeyDown || k.Rune() == 'j':
		a.st.Sel = min(a.st.Sel+1, len(helpRows)-1)
	case k.Key() == tcell.KeyUp || k.Rune() == 'k':
		a.st.Sel = max(a.st.Sel-1, 0)
	}
}

func (a *App) drawHelp(c ui.Canvas, w, h int) {
	bh := min(len(helpRows)+4, h)
	b := centered(w, h, 680, bh)
	b.frame(c)
	b.put(c, 2, 0, "help · meno keybindings & commands", ui.Blue, ui.BgPopup)
	b.putRight(c, 0, "v0.1", ui.FgDim, ui.BgPopup)
	visible := bh - 3
	start := min(max(a.st.Sel, 0), max(len(helpRows)-visible, 0))
	for i := 0; i < visible && start+i < len(helpRows); i++ {
		r := helpRows[start+i]
		y := 2 + i
		if r.key == "" {
			b.put(c, 2, y, r.desc, ui.FgWhite, ui.BgPopup)
			continue
		}
		b.put(c, 4, y, ui.Pad(r.key, 16), ui.Blue, ui.BgPopup)
		b.put(c, 20, y, r.desc, ui.FgSoft, ui.BgPopup)
	}
	b.hint(c, "j/k scroll · esc / q close")
}
