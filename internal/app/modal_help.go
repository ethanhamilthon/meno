package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

type helpRow struct{ key, desc string }

var helpRows = []helpRow{
	{"", "modes & navigation"},
	{"i a I A o O", "insert mode (input)"},
	{"esc", "normal mode (vim editing of the input)"},
	{"h j k l", "move; k/j cross between input and timeline"},
	{"w b e 0 $", "word / line motions (W B E too)"},
	{"gg G { }", "top / bottom / paragraph"},
	{"f t F T ; ,", "find a character in the line"},
	{"d c y + motion", "delete / change / yank (dw, ciw, yy, d$...)"},
	{"x s D C p u", "delete char, subst, p paste, u undo (ctrl+r redo)"},
	{"v / V / ctrl+v", "visual: char / line / block, anywhere"},
	{"o  iw  aw", "visual: swap ends, select word objects"},
	{"y d c p", "visual: yank / delete / change / replace (input only)"},
	{"ctrl+e/y/d/u", "scroll the timeline"},
	{":", "command mode (from normal mode)"},
	{"/name", "enabled skill, anywhere in the input"},
	{"enter", "send message"},
	{"shift+enter", "new line (also alt+enter, ctrl+j)"},
	{"← → ↑ ↓", "move the cursor in the input"},
	{"alt+← / alt+→", "move by word"},
	{"home / end", "start / end of line"},
	{"ctrl+c", "abort the running turn of this session"},
	{"", ""},
	{"", "direct commands"},
	{":model :settings :theme", "dialogs: model & effort (this session), display & timeouts, colors"},
	{":skills :mcp :providers", "dialogs: skills, MCP servers, API keys"},
	{":sessions", "switch sessions"},
	{":new", "start a clean session (others keep running)"},
	{":cd <path>", "change this session's working directory (~, -)"},
	{":reset", "restart goja vm"},
	{":compact", "summarize old context"},
	{":notify [off|question|done|test|command <cmd>]", "signal when the agent asks (question) or finishes (done)"},
	{":attach [path]", "attach an image: file or clipboard (also ctrl+v, normal-mode p)"},
	{":export", "save log to markdown"},
	{":stop", "stop this session's turn"},
	{":q  :q!", "exit meno (:q! also stops running turns)"},
}

func (a *App) helpKey(k *tcell.EventKey) {
	switch {
	case k.Rune() == 'q' || k.Key() == tcell.KeyEnter:
		a.closeOverlay()
	case k.Key() == tcell.KeyDown || k.Rune() == 'j':
		a.st.Sel++
	case k.Key() == tcell.KeyUp || k.Rune() == 'k':
		a.st.Sel = max(a.st.Sel-1, 0)
	}
}

func (a *App) drawHelp(c ui.Canvas, w, h int) {
	rows := append([]helpRow(nil), helpRows...)
	rows = append(rows, helpRow{desc: ""}, helpRow{desc: "usage · last request & session"})
	for _, line := range wrapParts(a.statsText(), " · ", max(1, modalWidth(w)-4)) {
		rows = append(rows, helpRow{desc: line})
	}
	bh := min(len(rows)+4, h)
	b := centered(w, h, 80, bh)
	b.frame(c)
	title := "help · meno keybindings & commands"
	if b.w < ui.TextWidth(title)+10 {
		title = "help · meno"
	}
	titleRow(title, "v0.1")(c, b)
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
			b.put(c, 2, y, r.desc, ui.FgWhite, ui.BgPopup)
			continue
		}
		b.put(c, 4, y, ui.Pad(r.key, keyWidth), ui.Blue, ui.BgPopup)
		b.put(c, 4+keyWidth, y, r.desc, ui.FgSoft, ui.BgPopup)
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
		case ui.TextWidth(cur)+ui.TextWidth(sep)+ui.TextWidth(p) <= width:
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
		final = append(final, ui.Wrap(l, width)...) // a single oversized part still wraps
	}
	return final
}
