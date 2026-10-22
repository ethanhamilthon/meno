package client

import (
	"encoding/json"
	"meno/internal/api"
)

func (m *Mirror) Apply(e api.Event) { m.ApplyUpdate(e) }

// ApplyUpdate reduces an ordered event exactly once. Recovery is sticky until Load.
func (m *Mirror) ApplyUpdate(e api.Event) (ProjectionUpdate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &m.State
	if e.Type == api.EventResync || e.BootID != "" && s.Version.BootID != "" && e.BootID != s.Version.BootID {
		s.NeedsResync = true
		return ProjectionUpdate{}, false
	}
	if s.NeedsResync || e.Seq > 0 && e.Seq <= s.Seq {
		return ProjectionUpdate{}, false
	}
	if e.Seq > 0 && s.Seq > 0 && e.Seq != s.Seq+1 {
		s.NeedsResync = true
		return ProjectionUpdate{}, false
	}
	if e.Seq > 0 {
		s.Seq = e.Seq
		s.Version.Revision = e.Seq
	}
	if e.BootID != "" {
		s.Version.BootID = e.BootID
	}
	var fields ProjectionFields
	var effects ProjectionEffects
	var turns []api.Turn
	if e.Type != api.EventRetry && e.Type != api.EventUsage && e.Type != api.EventHello && s.Retry != nil {
		s.Retry = nil
		fields |= RetryChanged
	}
	switch e.Type {
	case api.EventTextDelta, api.EventThinkDelta:
		var d api.TextDeltaData
		if json.Unmarshal(e.Data, &d) != nil {
			break
		}
		if e.Type == api.EventTextDelta {
			s.Live.Text += d.Text
			s.Phase = "writing"
		} else {
			s.Live.Thinking += d.Text
			s.Phase = "thinking"
		}
		fields |= LiveChanged | ActivityChanged
	case api.EventCallDelta, api.EventCallStart, api.EventCallEnd:
		fields |= m.applyCall(e, &effects)
	case api.EventTurnUpdated:
		var d api.TurnAddedData
		if json.Unmarshal(e.Data, &d) == nil && d.Turn.ID != "" {
			for i := range s.Turns {
				if s.Turns[i].ID == d.Turn.ID {
					s.Turns[i] = cloneTurns([]api.Turn{d.Turn})[0]
					turns = []api.Turn{d.Turn}
					fields |= TurnsChanged
					break
				}
			}
		}
	case api.EventTurnAdded:
		var d api.TurnAddedData
		if json.Unmarshal(e.Data, &d) != nil {
			break
		}
		if d.Turn.ID != "" && d.Turn.ID != "0" && hasTurn(s.Turns, d.Turn.ID) {
			break
		}
		s.Turns = append(s.Turns, cloneTurns([]api.Turn{d.Turn})[0])
		turns = []api.Turn{d.Turn}
		fields |= TurnsChanged
		if d.Turn.Role == "assistant" {
			s.Live.Text, s.Live.Thinking = "", ""
			fields |= LiveChanged
		}
	case api.EventStatus:
		var d api.StatusData
		if json.Unmarshal(e.Data, &d) != nil || !d.Running && staleRun(s, d.RequestID) {
			break
		}
		if d.Running && !s.Running {
			effects.Started = &d
			s.LastError = ""
			fields |= ErrorChanged
		}
		if d.Running && d.RequestID != "" {
			s.RequestID = d.RequestID
		}
		s.Running, s.Phase = d.Running, d.Phase
		fields |= ActivityChanged
	case api.EventRequest:
		s.Phase = "thinking"
		fields |= ActivityChanged
	case api.EventAskOpen:
		var d api.AskForm
		if json.Unmarshal(e.Data, &d) == nil {
			s.Ask = &d
			fields |= AskChanged
		}
	case api.EventAskClosed:
		var d api.AskClosedData
		if json.Unmarshal(e.Data, &d) == nil && s.Ask != nil && s.Ask.AskID == d.AskID {
			s.Ask = nil
			fields |= AskChanged
		}
	case api.EventRetry:
		var d api.RetryData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Retry = &api.RetryState{Attempt: d.Attempt, InMS: d.InMS, Reason: d.Reason}
			s.RetryMax = d.Max
			s.Live = api.LiveState{}
			s.StartedCalls = map[string]bool{}
			s.Phase = "retrying"
			fields |= RetryChanged | LiveChanged | ActivityChanged
			effects.Retry = &d
		}
	case api.EventUsage:
		var d api.UsageData
		if json.Unmarshal(e.Data, &d) == nil {
			m.applyUsage(d)
			fields |= UsageChanged
			effects.Usage = &d
		}
	case api.EventInjected:
		var d api.InjectedData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Queued = append([]string(nil), d.Queued...)
			fields |= QueueChanged
		}
	case api.EventError:
		var d api.ErrorData
		if json.Unmarshal(e.Data, &d) == nil && !staleRun(s, d.RequestID) {
			s.LastError = d.Message
			fields |= ErrorChanged
			effects.Error = &d
		}
	case api.EventDone, api.EventAborted:
		var d api.DoneData
		if json.Unmarshal(e.Data, &d) != nil || staleRun(s, d.RequestID) {
			break
		}
		s.Live = api.LiveState{}
		s.StartedCalls = map[string]bool{}
		s.Running = false
		s.Phase = ""
		s.Retry = nil
		fields |= LiveChanged | ActivityChanged | RetryChanged
		effects.Finished = true
		effects.Aborted = e.Type == api.EventAborted
		if !effects.Aborted {
			s.LastError = ""
			fields |= ErrorChanged
		}
	case api.EventSessionChanged, api.EventTasksChanged:
		fields |= m.applyChange(e)
	}
	if s.NeedsResync {
		return ProjectionUpdate{}, false
	}
	return m.projection(fields, false, turns, effects), true
}
