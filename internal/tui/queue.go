package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

const queueMaxShown = 3

// queueHeight is the rows of the panel listing messages waiting for the agent.
func (a *App) queueHeight() int {
	if !a.st.Running || a.st.Over != OverNone {
		return 0
	}
	n := len(a.st.Queued)
	if n == 0 {
		return 0
	}
	return 1 + min(n, queueMaxShown)
}

func (a *App) drawQueue(c render.Canvas, x, y, w, limit int) {
	items := a.st.Queued
	if len(items) == 0 || limit < 2 {
		return
	}
	shown := min(len(items), queueMaxShown, limit-1)
	bg := render.BgPopup
	style := func(fg tcell.Color) tcell.Style { return tcell.StyleDefault.Foreground(fg).Background(bg) }
	c.Fill(x, y, w, 1+shown, bg)
	title := "↳ queued · " + itoa(len(items)) + " · sent after the current step"
	hint := "↑ edit"
	if render.TextWidth(title)+render.TextWidth(hint)+6 > w {
		title = "↳ queued · " + itoa(len(items))
	}
	c.Text(x+2, y, max(0, w-4), title, style(render.Blue))
	if render.TextWidth(title)+render.TextWidth(hint)+6 <= w {
		c.Text(x+w-2-render.TextWidth(hint), y, render.TextWidth(hint), hint, style(render.FgDim))
	}
	for i := 0; i < shown; i++ {
		line := oneLine(a.labelsFromText(items[i]))
		if i == shown-1 && len(items) > shown {
			line += "  (+" + itoa(len(items)-shown) + " more)"
		}
		c.Text(x+2, y+1+i, max(0, w-4), render.Truncate(line, max(1, w-4)), style(render.FgSoft))
	}
}
