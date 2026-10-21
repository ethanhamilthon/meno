package app

import (
	"context"
	"encoding/json"
	"meno/internal/proto"
	"meno/internal/ui"
	"time"
)

type completionRead struct {
	sid     string
	through int64
	err     error
}

func (a *App) acknowledgeVisible() {
	s := a.st.Session
	if a.unfocused || a.cl == nil || a.st.Running || a.st.LiveText != "" || s == nil || a.st.Over != OverNone || a.st.Scroll != 0 || s.Completed <= s.ReadThrough || a.ackPending {
		return
	}
	a.ackPending = true
	id, through := s.ID(), s.Completed
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		data, _ := json.Marshal(map[string]int64{"through": through})
		_, err := a.cl.Resource(ctx, "POST", "/v1/sessions/"+id+"/read", "", proto.ResourceRequest{Data: data})
		a.postEvent(completionRead{id, through, err})
	})
}
func (a *App) globalCounts() []ui.Seg {
	running, asking, done := 0, 0, 0
	for _, s := range a.globalSessions {
		if s.NeedsYou {
			asking++
		} else if s.Running {
			running++
		}
		if s.Completed > s.ReadThrough {
			done++
		}
	}
	values := []struct {
		n     int
		name  string
		color ui.Seg
	}{
		{running, "running", ui.Seg{FG: ui.Green}}, {a.globalTasks, "tasks", ui.Seg{FG: ui.Purple}},
		{asking, "Asking", ui.Seg{FG: ui.Orange}}, {done, "done", ui.Seg{FG: ui.Blue}},
	}
	var out []ui.Seg
	for _, v := range values {
		if v.n == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, ui.Seg{Text: " | ", FG: ui.FgDim})
		}
		out = append(out, ui.Seg{Text: itoa(v.n) + " " + v.name, FG: v.color.FG})
	}
	return out
}
