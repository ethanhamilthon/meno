package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

type logRow struct {
	text string
	err  bool
}

// taskLogRows wraps the log to width: a long line continues on the next row
// instead of being cut, so nothing the task printed is hidden.
func (a *App) taskLogRows(width int) []logRow {
	var out []logRow
	for _, r := range a.st.TasksUI.Logs {
		text := strings.ReplaceAll(r.Text, "\t", "    ")
		for _, part := range render.Wrap(text, width) {
			out = append(out, logRow{part, r.Stderr})
		}
	}
	return out
}

// taskLogKey scrolls the open log. Up counts rows above the end, so new output
// does not move a view the user scrolled away from the end.
func (a *App) taskLogKey(k *tcell.EventKey) {
	v := &a.st.TasksUI
	_, h := a.scr.Size()
	w, _ := a.scr.Size()
	page := max(1, a.panelBox(w, h, 0, 0).h-4)
	switch {
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		v.Up++
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		v.Up = max(0, v.Up-1)
	case k.Key() == tcell.KeyPgUp || k.Key() == tcell.KeyCtrlB:
		v.Up += page
	case k.Key() == tcell.KeyPgDn || k.Key() == tcell.KeyCtrlF:
		v.Up = max(0, v.Up-page)
	case k.Key() == tcell.KeyHome || isKey(k, 'g'):
		v.Up = 1 << 30
	case k.Key() == tcell.KeyEnd || isKey(k, 'G'):
		v.Up = 0
	case isKey(k, 'y'):
		a.scr.SetClipboard([]byte(a.taskLogText()))
		a.notify("log copied")
	}
	v.Up = min(v.Up, max(0, len(a.taskLogRows(max(1, w-8)))-1))
}

func (a *App) taskLogText() string {
	var b strings.Builder
	for _, r := range a.st.TasksUI.Logs {
		b.WriteString(r.Text + "\n")
	}
	return b.String()
}
