package tui

import (
	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/llm"
	"time"
)

func retryFromWire(r *api.RetryState) *retryState {
	if r == nil {
		return nil
	}
	return &retryState{Attempt: r.Attempt, Max: r.Attempt, Until: time.Now().Add(time.Duration(r.InMS) * time.Millisecond), Reason: briefReason(r.Reason)}
}

// onProjection adapts accepted client state for drawing; no wire reduction occurs here.
func (a *App) onProjection(m projectionEv) {
	rt := a.rts[m.sid]
	if rt == nil || rt.generation != m.generation {
		return
	}
	u := m.update
	if u.Reset {
		a.setConnLost(m.sid, false)
	}
	a.applyProjection(rt, u)
}

func (a *App) applyProjection(rt *Runtime, u client.ProjectionUpdate) {
	f, v := u.Changed, u.Values
	if f&client.MetadataChanged != 0 {
		s := rt.Session
		s.id, s.Title, s.Cwd, s.Model = v.Session.ID, v.Session.Name, v.Cwd, v.Model
		if s.Title == "untitled" {
			s.Title = ""
		}
		s.Provider, s.Effort = v.Provider, v.Effort
		s.Completed, s.ReadThrough = v.Session.Completed, v.Session.ReadThrough
		if v.Session.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, v.Session.UpdatedAt); err == nil {
				s.UpdatedAt = t
			}
		}
		rt.Cwd = v.Cwd
	}
	if f&client.ActivityChanged != 0 && !(rt.sending > 0 && rt.Running && !v.Running && u.Reset) {
		rt.Running, rt.Phase, rt.RequestID = v.Running, v.Phase, v.RequestID
	}
	if f&client.TurnsChanged != 0 {
		if u.Reset {
			rt.Session.turns = nil
			rt.lines = lineCache{}
		}
		for _, t := range v.Turns {
			replaced := false
			for i := range rt.Session.turns {
				if t.ID != "" && rt.Session.turns[i].ID == t.ID {
					rt.Session.turns[i] = turnFromWire(t)
					replaced = true
					break
				}
			}
			if !replaced {
				rt.Session.add(t)
			}
		}
	}
	if f&client.LiveChanged != 0 {
		rt.LiveText, rt.LiveThink = v.Live.Text, v.Live.Thinking
		rt.LiveBase = rt.Session.Len()
		rt.rebuildLiveCalls(v.Live.Calls, v.StartedCalls, u.Effects.CallEnded)
	}
	if f&client.ActivityChanged != 0 && !v.Running {
		rt.dropEndedCalls()
	}
	if f&client.QueueChanged != 0 {
		rt.Queued = v.Queued
	}
	if f&client.TasksChanged != 0 {
		a.setTasks(rt, v.Tasks)
	}
	if f&client.AskChanged != 0 {
		a.setAsk(rt, v.Ask)
	}
	if f&client.RetryChanged != 0 {
		rt.Retry = retryFromWire(v.Retry)
		if rt.Retry != nil && v.RetryMax > 0 {
			rt.Retry.Max = v.RetryMax
		}
	}
	if f&client.UsageChanged != 0 {
		rt.Spent, rt.Context, rt.Unpriced = v.Spent, v.Context, v.Unpriced
		rt.Usage = usageFromProjection(v.TotalUsage)
		rt.LastUsage = usageFromProjection(v.LastUsage)
	}
	if f&client.ErrorChanged != 0 {
		rt.LastErr = v.LastError != ""
	}
	rt.Saved = true
	a.projectionEffects(rt, u.Effects)
}

func usageFromProjection(d api.UsageData) llm.Usage {
	return llm.Usage{In: int(d.InputTokens), Out: int(d.OutputTokens), Cached: int(d.Cached), CacheWrite: int(d.CacheWrite), CacheReported: d.CacheKnown}
}
