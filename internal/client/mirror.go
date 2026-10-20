package client

import (
	"encoding/json"
	"sync"

	"meno/internal/proto"
)

// Mirror is the client-side session state rebuilt from a snapshot and ordered events.
type Mirror struct {
	mu    sync.RWMutex
	State MirrorState
}

// MirrorState contains view-ready session and live-run state.
type MirrorState struct {
	Session                      proto.SessionItem
	Cwd, Model, Phase, RequestID string
	Modes                        []string
	Running                      bool
	Seq                          uint64
	Turns                        []proto.Turn
	Live                         proto.LiveState
	Ask                          *proto.AskForm
	Retry                        *proto.RetryState
	Metrics                      proto.Metrics
	Queued                       []string
	Provider, Effort             string
	Spent                        float64
	Context                      int
	Unpriced                     bool
	Tasks                        []proto.Task
	NeedsResync                  bool
}

// Load replaces all state from a consistent snapshot.
func (m *Mirror) Load(s proto.Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.State = MirrorState{Session: s.Session, Cwd: s.Cwd, Model: s.Model, Modes: append([]string(nil), s.Modes...), Phase: s.Phase, RequestID: s.RequestID, Running: s.Running, Seq: s.Seq, Turns: append([]proto.Turn(nil), s.Turns...), Live: s.Live, Ask: s.Ask, Retry: s.Retry, Metrics: s.Metrics, Queued: append([]string(nil), s.Queued...), Provider: s.Provider, Effort: s.Effort, Spent: s.Spent, Context: s.Context, Unpriced: s.Unpriced, Tasks: append([]proto.Task(nil), s.Tasks...)}
}

// Snapshot returns an isolated copy of the current view state.
func (m *Mirror) Snapshot() MirrorState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.State
	s.Queued = append([]string(nil), s.Queued...)
	s.Modes = append([]string(nil), s.Modes...)
	s.Turns = append([]proto.Turn(nil), s.Turns...)
	if s.Ask != nil {
		ask := *s.Ask
		ask.Questions = append([]proto.AskQuestion(nil), s.Ask.Questions...)
		s.Ask = &ask
	}
	if s.Retry != nil {
		retry := *s.Retry
		s.Retry = &retry
	}
	s.Tasks = append([]proto.Task(nil), s.Tasks...)
	s.Live.Calls = append([]proto.CallState(nil), s.Live.Calls...)
	return s
}

// Apply updates view state from one stream event. Sequence gaps request a fresh snapshot.
func (m *Mirror) Apply(e proto.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &m.State
	if e.Type == proto.EventResync {
		s.NeedsResync = true
		return
	}
	if e.Seq > 0 && s.Seq > 0 && e.Seq != s.Seq+1 {
		s.NeedsResync = true
		return
	}
	if e.Seq > s.Seq {
		s.Seq = e.Seq
	}
	switch e.Type {
	case proto.EventTextDelta:
		var d proto.TextDeltaData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Live.Text += d.Text
		}
	case proto.EventThinkDelta:
		var d proto.TextDeltaData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Live.Thinking += d.Text
		}
	case proto.EventCallStart:
		var d proto.CallStartData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Live.Calls = append(s.Live.Calls, proto.CallState{CallID: d.CallID, Command: d.Command, Title: d.Title})
		}
	case proto.EventCallDelta:
		var d proto.CallDeltaData
		if json.Unmarshal(e.Data, &d) == nil {
			if c := m.call(d.CallID); c != nil {
				c.Delta += d.Delta
			}
		}
	case proto.EventCallEnd:
		var d proto.CallEndData
		if json.Unmarshal(e.Data, &d) == nil {
			if c := m.call(d.CallID); c != nil {
				c.Output = d.Output
				c.Exit = &d.Exit
				c.ElapsedMS = d.ElapsedMS
				c.TaskID = d.TaskID
			}
		}
	case proto.EventTurnAdded:
		var d proto.TurnAddedData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Turns = append(s.Turns, d.Turn)
			s.Live = proto.LiveState{Calls: []proto.CallState{}}
		}
	case proto.EventStatus:
		var d proto.StatusData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Running = d.Running
			s.Phase = d.Phase
		}
	case proto.EventAskOpen:
		var d proto.AskForm
		if json.Unmarshal(e.Data, &d) == nil {
			s.Ask = &d
		}
	case proto.EventAskClosed:
		var d proto.AskClosedData
		if json.Unmarshal(e.Data, &d) == nil && s.Ask != nil && s.Ask.AskID == d.AskID {
			s.Ask = nil
		}
	case proto.EventRetry:
		var d proto.RetryState
		if json.Unmarshal(e.Data, &d) == nil {
			s.Retry = &d
		}
	case proto.EventUsage:
		var d proto.UsageData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Metrics.InputTokens += d.InputTokens
			s.Metrics.OutputTokens += d.OutputTokens
			s.Metrics.Cost += d.Cost
			s.Context = int(d.InputTokens + d.OutputTokens)
			if d.Priced {
				s.Spent += d.Cost
			} else if d.InputTokens+d.OutputTokens > 0 {
				s.Unpriced = true
			}
		}
	case proto.EventTasksChanged:
		s.NeedsResync = true
	case proto.EventInjected:
		var d proto.InjectedData
		if json.Unmarshal(e.Data, &d) == nil {
			s.Queued = append([]string(nil), d.Queued...)
		}
	case proto.EventDone, proto.EventAborted:
		s.Running = false
		s.Phase = ""
		s.Retry = nil
	case proto.EventSessionChanged:
		var d proto.SessionItem
		if json.Unmarshal(e.Data, &d) == nil {
			s.Session = d
			s.Cwd = d.Cwd
			s.Model = d.Model
		}
	}
}
func (m *Mirror) call(id string) *proto.CallState {
	for i := range m.State.Live.Calls {
		if m.State.Live.Calls[i].CallID == id {
			return &m.State.Live.Calls[i]
		}
	}
	return nil
}
