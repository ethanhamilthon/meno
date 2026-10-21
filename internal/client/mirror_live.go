package client

import (
	"encoding/json"
	"meno/internal/proto"
)

func (m *Mirror) call(id string) *proto.CallState {
	s := &m.State
	for i := range s.Live.Calls {
		if id != "" && s.Live.Calls[i].CallID == id {
			return &s.Live.Calls[i]
		}
	}
	if n := len(s.Live.Calls); n > 0 && s.Live.Calls[n-1].CallID == "" && !s.StartedCalls[""] {
		s.Live.Calls[n-1].CallID = id
		return &s.Live.Calls[n-1]
	}
	s.Live.Calls = append(s.Live.Calls, proto.CallState{CallID: id})
	return &s.Live.Calls[len(s.Live.Calls)-1]
}

func (m *Mirror) applyCall(e proto.Event, effects *ProjectionEffects) ProjectionFields {
	s := &m.State
	if s.StartedCalls == nil {
		s.StartedCalls = map[string]bool{}
	}
	switch e.Type {
	case proto.EventCallDelta:
		var d proto.CallDeltaData
		if json.Unmarshal(e.Data, &d) != nil {
			return 0
		}
		c := m.call(d.CallID)
		c.Delta += d.Delta
		if !s.StartedCalls[d.CallID] {
			c.Command += d.Delta
		}
		s.Phase = "writing"
	case proto.EventCallStart:
		var d proto.CallStartData
		if json.Unmarshal(e.Data, &d) != nil {
			return 0
		}
		c := m.call(d.CallID)
		c.Command = d.Command
		c.Title = d.Title
		s.StartedCalls[d.CallID] = true
		s.Phase = "running"
		effects.CallStarted = &d
	case proto.EventCallEnd:
		var d proto.CallEndData
		if json.Unmarshal(e.Data, &d) != nil {
			return 0
		}
		calls := s.Live.Calls[:0]
		for _, c := range s.Live.Calls {
			if c.CallID != d.CallID {
				calls = append(calls, c)
			}
		}
		s.Live.Calls = calls
		delete(s.StartedCalls, d.CallID)
		if len(calls) == 0 {
			s.Phase = "thinking"
		}
		effects.CallEnded = &d
	}
	return LiveChanged | ActivityChanged
}

func (m *Mirror) applyUsage(d proto.UsageData) {
	s := &m.State
	s.Metrics.InputTokens += d.InputTokens
	s.Metrics.OutputTokens += d.OutputTokens
	s.Metrics.Cost += d.Cost
	s.LastUsage = d
	s.TotalUsage.InputTokens += d.InputTokens
	s.TotalUsage.OutputTokens += d.OutputTokens
	s.TotalUsage.Cached += d.Cached
	s.TotalUsage.CacheWrite += d.CacheWrite
	s.TotalUsage.CacheKnown = s.TotalUsage.CacheKnown || d.CacheKnown
	s.Context = int(d.InputTokens + d.OutputTokens)
	if d.Priced {
		s.Spent += d.Cost
	} else if d.InputTokens+d.OutputTokens > 0 {
		s.Unpriced = true
	}
}
