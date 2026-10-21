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
	Version                      proto.SessionVersion
	LastError                    string
}

// Load replaces all state from a consistent snapshot.
func (m *Mirror) Load(s proto.Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.Version.Revision == 0 {
		s.Version.Revision = s.Seq
	}
	// A snapshot older than the state already applied would roll it back.
	if m.State.Version.Newer(s.Version) {
		return
	}
	m.State = MirrorState{Session: s.Session, Cwd: s.Cwd, Model: s.Model, Phase: s.Phase, RequestID: s.RequestID, Running: s.Running, Seq: s.Seq, Turns: append([]proto.Turn(nil), s.Turns...), Live: s.Live, Ask: s.Ask, Retry: s.Retry, Metrics: s.Metrics, Queued: append([]string(nil), s.Queued...), Provider: s.Provider, Effort: s.Effort, Spent: s.Spent, Context: s.Context, Unpriced: s.Unpriced, Tasks: append([]proto.Task(nil), s.Tasks...), Version: s.Version, LastError: s.LastError}
	m.State.Live = cloneLiveState(s.Live)
	if s.Ask != nil {
		ask := *s.Ask
		ask.Questions = append([]proto.AskQuestion(nil), s.Ask.Questions...)
		m.State.Ask = &ask
	}
	if s.Retry != nil {
		retry := *s.Retry
		m.State.Retry = &retry
	}
}

// Snapshot returns an isolated copy of the current view state.
func (m *Mirror) Snapshot() MirrorState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.State
	s.Queued = append([]string(nil), s.Queued...)
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
	if e.BootID != "" && s.Version.BootID != "" && e.BootID != s.Version.BootID {
		s.NeedsResync = true
		return
	}
	if e.Seq > 0 && e.Seq <= s.Seq {
		return
	}
	if e.Seq > 0 && s.Seq > 0 && e.Seq != s.Seq+1 {
		s.NeedsResync = true
		return
	}
	if e.Seq > s.Seq {
		s.Seq = e.Seq
		s.Version.Revision = e.Seq
		if e.BootID != "" {
			s.Version.BootID = e.BootID
		}
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
			calls := s.Live.Calls[:0]
			for _, c := range s.Live.Calls {
				if c.CallID != d.CallID {
					calls = append(calls, c)
				}
			}
			s.Live.Calls = calls
		}
	case proto.EventTurnAdded:
		var d proto.TurnAddedData
		if json.Unmarshal(e.Data, &d) == nil {
			if d.Turn.ID != "" && d.Turn.ID != "0" && hasTurn(s.Turns, d.Turn.ID) {
				break // replayed after a snapshot that already holds it
			}
			s.Turns = append(s.Turns, d.Turn)
			if d.Turn.Role == "assistant" {
				s.Live.Text, s.Live.Thinking = "", ""
			}
		}
	case proto.EventStatus:
		var d proto.StatusData
		if json.Unmarshal(e.Data, &d) == nil {
			if !d.Running && staleRun(s, d.RequestID) {
				break
			}
			if d.Running && d.RequestID != "" {
				s.RequestID = d.RequestID
			}
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
			s.Live = proto.LiveState{}
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
	case proto.EventError:
		var d proto.ErrorData
		if json.Unmarshal(e.Data, &d) == nil {
			s.LastError = d.Message
		}
	case proto.EventDone, proto.EventAborted:
		var d proto.DoneData
		if json.Unmarshal(e.Data, &d) == nil && staleRun(s, d.RequestID) {
			break
		}
		s.Live = proto.LiveState{}
		s.Running = false
		s.Phase = ""
		s.Retry = nil
		if e.Type == proto.EventDone {
			s.LastError = "" // a success clears an old failure
		}
	case proto.EventSessionChanged:
		// The event names changed fields; it does not carry the new values.
		// Keep what is known and ask for a fresh snapshot.
		s.NeedsResync = true
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

func hasTurn(turns []proto.Turn, id string) bool {
	for _, t := range turns {
		if t.ID == id {
			return true
		}
	}
	return false
}

// staleRun reports an event of a run that is no longer the active one: a late
// terminal event of an old run must not end a request that started after it.
func staleRun(s *MirrorState, rid string) bool {
	return rid != "" && s.RequestID != "" && s.Running && rid != s.RequestID
}

// cloneLiveState copies the call list: the mirror changes its calls in place
// while the caller of Load still reads the snapshot it passed in.
func cloneLiveState(l proto.LiveState) proto.LiveState {
	l.Calls = append([]proto.CallState(nil), l.Calls...)
	return l
}
