package app

import (
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

func (a *App) onAgent(e agent.Ev) {
	if e.Kind != agent.EvRetry && e.Kind != agent.EvUsage {
		a.st.Retry = nil
	}
	switch e.Kind {
	case agent.EvRetry:
		// The failed request's partial output is void: the request starts over.
		a.clearLive()
		a.st.Phase = "retrying"
		a.st.Retry = &retryState{Attempt: e.Attempt, Max: e.Max, Until: time.Now().Add(e.Delay), Reason: briefReason(e.Err)}
		a.notify("retry " + itoa(e.Attempt) + "/" + itoa(e.Max) + " in " + fmtWait(e.Delay) + " — " + a.st.Retry.Reason)
		return
	}
	switch e.Kind {
	case agent.EvThink, agent.EvText, agent.EvCode, agent.EvJSStart, agent.EvJSEnd:
		a.st.LiveBase = e.Turns
	}
	switch e.Kind {
	case agent.EvThink:
		a.st.LiveThink += e.Thinking
		a.st.Phase = "thinking"
	case agent.EvText:
		a.st.LiveText += e.Text
		a.st.Phase = "writing"
	case agent.EvCode:
		a.st.Phase = "writing"
		a.liveCall(e.CallID).Code = e.Code
	case agent.EvJSStart:
		lc := a.liveCall(e.CallID)
		lc.Code, lc.Running = e.Code, true
		lc.Started, lc.Limit = time.Now(), e.Limit
		a.st.Phase = "running"
	case agent.EvJSEnd:
		// The finished call (and the text before it) is in the session now.
		a.dropLiveCall(e.CallID)
		a.st.RunJS++
		a.st.LiveText, a.st.LiveThink = "", ""
		a.st.Phase = "thinking"
	case agent.EvInjected:
		a.clearLive()
		a.st.Phase = "thinking"
	case agent.EvDone, agent.EvAborted:
		a.clearLive()
		a.st.Phase = ""
	case agent.EvUsage:
		a.st.RunTurns++
		u := e.Usage
		a.st.Usage.In += u.In
		a.st.Usage.Out += u.Out
		a.st.Usage.Cached += u.Cached
		a.st.Usage.CacheWrite += u.CacheWrite
		a.st.Usage.CacheReported = a.st.Usage.CacheReported || u.CacheReported
		a.st.LastUsage = u
		a.st.Context = u.In + u.Out
		p, model := e.Provider, e.Model
		if model == "" {
			p, _ = a.currentProvider()
			_, model, _ = a.selection()
		}
		record := agent.RequestUsage{Provider: p.Name, Model: model, Usage: u, Time: time.Now().UTC()}
		if info, ok := llm.LookupMetadata(a.st.Pricing, p, model); ok && info.CanPrice(u) {
			cost := info.Cost(u)
			record.Cost = &cost
			a.st.Spent += cost
		} else if u.In > 0 || u.Out > 0 || u.Cached > 0 || u.CacheWrite > 0 {
			a.st.Unpriced = true
		}
		if a.st.Session != nil {
			a.st.Session.Requests = append(a.st.Session.Requests, record)
		}
	case agent.EvRequest:
		if e.Fp != nil {
			a.st.Fps = append(a.st.Fps, *e.Fp)
			if len(a.st.Fps) > 2 {
				a.st.Fps = a.st.Fps[1:]
			}
		}
	case agent.EvError:
		a.notifyErr(e.Err)
	}
}
