package app

import (
	"encoding/json"
	"time"

	"meno/internal/llm"
	"meno/internal/proto"
)

// onSnapshot replaces what a session shows with the daemon's snapshot.
func (a *App) onSnapshot(m snapshotEv) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	sn := m.snap
	rt.Session.load(sn)
	rt.Cwd = sn.Cwd
	// A snapshot taken before my message reached the daemon must not stop a run
	// I just started.
	if !(rt.sending > 0 && rt.Running && !sn.Running) {
		rt.Running, rt.Phase, rt.RequestID = sn.Running, sn.Phase, sn.RequestID
		rt.Queued = append([]string(nil), sn.Queued...)
	}
	rt.LiveText, rt.LiveThink, rt.LiveCalls = sn.Live.Text, sn.Live.Thinking, nil
	rt.LiveBase = len(rt.Session.turns)
	for _, c := range sn.Live.Calls {
		lc := liveCall{ID: c.CallID, Code: c.Command, Title: c.Title, Running: c.LimitMS > 0 || c.ElapsedMS > 0}
		if c.Delta != "" && lc.Code == "" {
			lc.Code = c.Delta
		}
		rt.LiveCalls = append(rt.LiveCalls, lc)
	}
	rt.Spent, rt.Context, rt.Unpriced = sn.Spent, sn.Context, sn.Unpriced
	rt.Saved = true
	a.setTasks(rt, sn.Tasks)
	a.setAsk(rt, sn.Ask)
	rt.Retry = retryFromWire(sn.Retry)
}

func retryFromWire(r *proto.RetryState) *retryState {
	if r == nil {
		return nil
	}
	return &retryState{Attempt: r.Attempt, Max: r.Attempt, Until: time.Now().Add(time.Duration(r.InMS) * time.Millisecond), Reason: briefReason(r.Reason)}
}

// onWire applies one event of a session stream to its runtime.
func (a *App) onWire(m wireEv) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	a.inRT(m.sid, func() { a.applyWire(rt, m.ev) })
}

func decode[T any](raw json.RawMessage) (T, bool) {
	var v T
	return v, json.Unmarshal(raw, &v) == nil
}

func (a *App) applyWire(rt *Runtime, e proto.Event) {
	if e.Type != proto.EventRetry && e.Type != proto.EventUsage && e.Type != proto.EventHello {
		rt.Retry = nil
	}
	switch e.Type {
	case proto.EventTextDelta:
		if d, ok := decode[proto.TextDeltaData](e.Data); ok {
			rt.LiveBase = rt.Session.Len()
			rt.LiveText += d.Text
			rt.Phase = "writing"
		}
	case proto.EventThinkDelta:
		if d, ok := decode[proto.TextDeltaData](e.Data); ok {
			rt.LiveBase = rt.Session.Len()
			rt.LiveThink += d.Text
			rt.Phase = "thinking"
		}
	case proto.EventCallDelta:
		if d, ok := decode[proto.CallDeltaData](e.Data); ok {
			rt.LiveBase = rt.Session.Len()
			rt.Phase = "writing"
			a.liveCall(d.CallID).Code += d.Delta
		}
	case proto.EventCallStart:
		if d, ok := decode[proto.CallStartData](e.Data); ok {
			lc := a.liveCall(d.CallID)
			lc.Code, lc.Title, lc.Running = d.Command, d.Title, true
			if d.Title != "" {
				rt.Title = d.Title
			}
			lc.Started, lc.Limit = time.Now(), time.Duration(d.LimitMS)*time.Millisecond
			rt.Phase = "running"
		}
	case proto.EventCallEnd:
		if d, ok := decode[proto.CallEndData](e.Data); ok {
			a.dropLiveCall(d.CallID)
			rt.RunJS++
			rt.LiveText, rt.LiveThink = "", ""
			rt.Phase = "thinking"
		}
	case proto.EventTurnAdded:
		if d, ok := decode[proto.TurnAddedData](e.Data); ok {
			rt.Session.add(d.Turn)
			if d.Turn.Role == "assistant" && d.Turn.Content != "" || d.Turn.Run != nil {
				rt.LiveText, rt.LiveThink = "", ""
			}
		}
	case proto.EventStatus:
		if d, ok := decode[proto.StatusData](e.Data); ok {
			if d.Running && !rt.Running {
				rt.RunStart, rt.RunTurns, rt.RunJS, rt.RunUsage0 = time.Now(), 0, 0, rt.Usage
				rt.Title = ""
			}
			rt.Running, rt.Phase = d.Running, d.Phase
		}
	case proto.EventRequest:
		rt.Phase = "thinking"
	case proto.EventUsage:
		if d, ok := decode[proto.UsageData](e.Data); ok {
			a.addUsage(rt, d)
		}
	case proto.EventRetry:
		if d, ok := decode[proto.RetryData](e.Data); ok {
			a.clearLive()
			rt.Phase = "retrying"
			rt.Retry = &retryState{Attempt: d.Attempt, Max: d.Max, Until: time.Now().Add(time.Duration(d.InMS) * time.Millisecond), Reason: briefReason(d.Reason)}
			a.notify("retry " + itoa(d.Attempt) + "/" + itoa(d.Max) + " in " + fmtWait(time.Duration(d.InMS)*time.Millisecond) + " — " + rt.Retry.Reason)
		}
	case proto.EventInjected:
		if d, ok := decode[proto.InjectedData](e.Data); ok {
			rt.Queued = append([]string(nil), d.Queued...)
		}
		a.clearLive()
		rt.Phase = "thinking"
	case proto.EventError:
		if d, ok := decode[proto.ErrorData](e.Data); ok {
			rt.LastErr = true
			a.notifyErr(d.Message)
		}
	case proto.EventAborted:
		a.clearLive()
		rt.Phase, rt.Running = "", false
		a.markUnseen(rt.ID())
		a.signal("")
	case proto.EventDone:
		a.clearLive()
		rt.Phase, rt.Running = "", false
		a.markUnseen(rt.ID())
		a.signal("done")
		a.refreshSession(rt.ID()) // the daemon names a session after its first message
	case proto.EventAskOpen:
		if d, ok := decode[proto.AskForm](e.Data); ok {
			a.setAsk(rt, &d)
		}
	case proto.EventAskClosed:
		if d, ok := decode[proto.AskClosedData](e.Data); ok && rt.Ask != nil && rt.Ask.ID == d.AskID {
			a.closeAsk(rt.Ask)
		}
	case proto.EventSessionChanged, proto.EventTasksChanged:
		a.refreshSession(rt.ID())
	}
}

// addUsage counts one answered request into the totals of the session.
func (a *App) addUsage(rt *Runtime, d proto.UsageData) {
	rt.RunTurns++
	u := llm.Usage{In: int(d.InputTokens), Out: int(d.OutputTokens), Cached: int(d.Cached), CacheWrite: int(d.CacheWrite), CacheReported: d.CacheKnown}
	rt.Usage.In += u.In
	rt.Usage.Out += u.Out
	rt.Usage.Cached += u.Cached
	rt.Usage.CacheWrite += u.CacheWrite
	rt.Usage.CacheReported = rt.Usage.CacheReported || u.CacheReported
	rt.LastUsage = u
	rt.Context = u.In + u.Out
	if d.Priced {
		rt.Spent += d.Cost
	} else if u.In > 0 || u.Out > 0 {
		rt.Unpriced = true
	}
}
