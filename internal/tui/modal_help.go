package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
	"meno/internal/version"
)

type helpRow struct{ key, desc string }

var helpRows = []helpRow{
	{"", "modes & navigation"}, {"/", "commands, from any word start"}, {"Esc", "close list / keep INPUT"},
	{"/skill:<name>", "insert a skill reference"},
	{"Ctrl+C", "cancel active work"},
	{"/context", "context, tokens, cache and cost"}, {"/new", "new session"},
	{"/fork", "fork from a message"}, {"/sessions", "switch sessions"}, {"/login", "providers and accounts"},
}

func (a *App) helpKey(k *tcell.EventKey) {
	switch {
	case isKey(k, 'q') || k.Key() == tcell.KeyEnter:
		a.closeOverlay()
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		a.st.Sel++
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		a.st.Sel = max(a.st.Sel-1, 0)
	}
}

func (a *App) drawHelp(c render.Canvas, w, h int) {
	rows := append([]helpRow(nil), helpRows...)
	rows = append(rows, helpRow{desc: ""}, helpRow{desc: "usage · last request & session"})
	for _, line := range wrapParts(a.statsText(), " · ", max(1, modalWidth(w)-4)) {
		rows = append(rows, helpRow{desc: line})
	}
	bh := min(len(rows)+4, h)
	b := centered(w, h, 80, bh)
	b.frame(c)
	title := "help · meno keybindings & commands"
	if b.w < render.TextWidth(title)+10 {
		title = "help · meno"
	}
	titleRow(title, version.Label())(c, b)
	keyWidth := 16
	if b.w < 60 {
		keyWidth = 10
	}
	visible := max(0, b.h-3)
	a.st.Sel = min(a.st.Sel, max(0, len(rows)-visible))
	start := max(a.st.Sel, 0)
	for i := 0; i < visible && start+i < len(rows); i++ {
		r := rows[start+i]
		y := 2 + i
		if r.key == "" {
			b.put(c, 2, y, r.desc, render.FgWhite, render.BgPopup)
			continue
		}
		b.put(c, 4, y, render.Pad(r.key, keyWidth), render.Blue, render.BgPopup)
		b.put(c, 4+keyWidth, y, r.desc, render.FgSoft, render.BgPopup)
	}
	b.hint(c, "j/k scroll · esc / q close")
}

// wrapParts packs the sep-separated parts into lines of at most width cells
// without cutting a part in the middle.
func wrapParts(s, sep string, width int) []string {
	var out []string
	cur := ""
	for _, p := range strings.Split(s, sep) {
		switch {
		case cur == "":
			cur = p
		case render.TextWidth(cur)+render.TextWidth(sep)+render.TextWidth(p) <= width:
			cur += sep + p
		default:
			out = append(out, cur)
			cur = p
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	var final []string
	for _, l := range out {
		final = append(final, render.Wrap(l, width)...) // a single oversized part still wraps
	}
	return final
}
