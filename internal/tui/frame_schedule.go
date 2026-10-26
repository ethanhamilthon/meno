package tui

import (
	"meno/internal/client"
	"time"
)

const streamFrameInterval = time.Second / 30

type frameSchedule struct{ last time.Time }

func (s *frameSchedule) delay(now time.Time, urgent bool) time.Duration {
	if urgent || s.last.IsZero() {
		return 0
	}
	return max(0, streamFrameInterval-now.Sub(s.last))
}
func (a *App) streamOnly(msg any) bool {
	m, ok := msg.(projectionEv)
	return ok && m.update.Changed != 0 && m.update.Changed & ^client.LiveChanged == 0
}
func (a *App) frameUrgent(msg any) bool { return a.eventVisible(msg) && !a.streamOnly(msg) }
func (a *App) eventVisible(msg any) bool {
	switch m := msg.(type) {
	case tick, themeLoaded:
		return false
	case projectionEv:
		rt := a.rts[m.sid]
		if rt == nil || rt.generation != m.generation {
			return false
		}
		if m.update.Changed == 0 && !m.update.Reset {
			return false
		}
		return rt == a.st.Runtime || m.update.Changed & ^client.LiveChanged != 0
	case tasksLoaded:
		return m.sid == a.st.Session.ID() && m.id == a.st.TasksUI.LogID && !sameTaskLogs(a.st.TasksUI.Logs, m.logs)
	}
	return true
}
func sameTaskLogs(a, b []taskLogLine) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
