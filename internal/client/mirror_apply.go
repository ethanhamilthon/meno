package client

import (
	"encoding/json"
	"meno/internal/proto"
)

func (m *Mirror) Apply(e proto.Event) { m.ApplyUpdate(e) }

// ApplyUpdate reduces an ordered event exactly once. Recovery is sticky until Load.
func (m *Mirror) ApplyUpdate(e proto.Event) (ProjectionUpdate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &m.State
	if e.Type == proto.EventResync || e.BootID != "" && s.Version.BootID != "" && e.BootID != s.Version.BootID {
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
	var turns []proto.Turn
	if e.Type != proto.EventRetry && e.Type != proto.EventUsage && e.Type != proto.EventHello && s.Retry != nil {
		s.Retry = nil
		fields |= RetryChanged
	}
	switch e.Type {
	case proto.EventTextDelta, proto.EventThinkDelta:
		var d proto.TextDeltaData
		if json.Unmarshal(e.Data, &d) != nil {
			break
		}
		if e.Type == proto.EventTextDelta {
			s.Live.Text += d.Text
			s.Phase = "writing"
		} else {
			s.Live.Thinking += d.Text
			s.Phase = "thinking"
		}
		fields |= LiveChanged | ActivityChanged
	case proto.EventCallDelta, proto.EventCallStart, proto.EventCallEnd:
		fields |= m.applyCall(e, &effects)
	case proto.EventTurnUpdated:
		var d proto.TurnAddedData
		if json.Unmarshal(e.Data, &d) == nil && d.Turn.ID != "" {
			for i := range s.Turns {
				if s.Turns[i].ID == d.Turn.ID {
					s.Turns[i] = cloneTurns([]proto.Turn{d.Turn})[0]
					turns = []proto.Turn{d.Turn}
					fields |= TurnsChanged
					break
				}
			}
		}
	case proto.EventTurnAdded:
		var d proto.TurnAddedData
		if json.Unmarshal(e.Data, &d) != nil {
			break
		}
		if d.Turn.ID != "" && d.Turn.ID != "0" && hasTurn(s.Turns, d.Turn.ID) {
			break
		}
		s.Turns = append(s.Turns, cloneTurns([]proto.Turn{d.Turn})[0])
		turns = []proto.Turn{d.Turn}
		fields |= TurnsChanged
		if d.Turn.Role == "assistant" {
			s.Live.Text, s.Live.Thinking = "", ""
			fields |= LiveChanged
		}
	case proto.EventStatus:
		var d proto.StatusData
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
	case proto.EventRequest:
		s.Phase = "thinking"
		fields |= ActivityChanged
	case proto.EventAskOpen:
		var d proto.AskForm
		if json.Unmarshal(e.Data, &d) == nil {
			s.Ask = &d
			fields |= AskChanged
		}
	case proto.EventAskClosed:
		var d proto.AskClosedData
		if json.Unmarshal(e.Data, &d) == nil && s.Ask != nil && s.Ask.AskID == d.AskID {
			s.Ask = nil
			fields |= AskChanged
		}
	case proto.EventRetry:
		var d proto.RetryData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Retry = &proto.RetryState{Attempt: d.Attempt, InMS: d.InMS, Reason: d.Reason}
			s.RetryMax = d.Max
			s.Live = proto.LiveState{}
			s.StartedCalls = map[string]bool{}
			s.Phase = "retrying"
			fields |= RetryChanged | LiveChanged | ActivityChanged
			effects.Retry = &d
		}
	case proto.EventUsage:
		var d proto.UsageData
		if json.Unmarshal(e.Data, &d) == nil {
			m.applyUsage(d)
			fields |= UsageChanged
			effects.Usage = &d
		}
	case proto.EventInjected:
		var d proto.InjectedData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Queued = append([]string(nil), d.Queued...)
			fields |= QueueChanged
		}
	case proto.EventError:
		var d proto.ErrorData
		if json.Unmarshal(e.Data, &d) == nil && !staleRun(s, d.RequestID) {
			s.LastError = d.Message
			fields |= ErrorChanged
			effects.Error = &d
		}
	case proto.EventDone, proto.EventAborted:
		var d proto.DoneData
		if json.Unmarshal(e.Data, &d) != nil || staleRun(s, d.RequestID) {
			break
		}
		s.Live = proto.LiveState{}
		s.StartedCalls = map[string]bool{}
		s.Running = false
		s.Phase = ""
		s.Retry = nil
		fields |= LiveChanged | ActivityChanged | RetryChanged
		effects.Finished = true
		effects.Aborted = e.Type == proto.EventAborted
		if !effects.Aborted {
			s.LastError = ""
			fields |= ErrorChanged
		}
	case proto.EventSessionChanged, proto.EventTasksChanged:
		fields |= m.applyChange(e)
	}
	if s.NeedsResync {
		return ProjectionUpdate{}, false
	}
	return m.projection(fields, false, turns, effects), true
}
