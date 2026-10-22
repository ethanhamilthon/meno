package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
	"meno/internal/render"
)

// setTasks keeps the background tasks of a session as the daemon listed them.
func (a *App) setTasks(rt *Runtime, ts []api.Task) {
	rt.Tasks = rt.Tasks[:0]
	for _, t := range ts {
		rt.Tasks = append(rt.Tasks, taskRow{ID: t.ID, Name: t.Name, Command: t.Command, Status: t.Status, Started: t.StartedAt, Exit: t.Exit})
	}
}

func taskRunning(s string) bool { return s == "starting" || s == "running" }

// bgCount is the number of running background tasks of a session, as last reported.
func (a *App) bgCount(sid string) int {
	rt := a.rts[sid]
	if rt == nil {
		return 0
	}
	n := 0
	for _, t := range rt.Tasks {
		if taskRunning(t.Status) {
			n++
		}
	}
	return n
}

// bgAny reports whether any session has background work, which keeps the
// purple marker blinking.
func (a *App) bgAny() bool {
	for _, rt := range a.rtList {
		if a.bgCount(rt.ID()) > 0 {
			return true
		}
	}
	return false
}

// bgColor is the blinking purple of the background-work marker.
func bgColor() tcell.Color {
	if blinkOn() {
		return render.Purple
	}
	return render.FgDim
}

// bgBadge is the marker text for n units of background work, "" for none.
func bgBadge(n int) string {
	if n <= 0 {
		return ""
	}
	return "◆ " + itoa(n)
}
