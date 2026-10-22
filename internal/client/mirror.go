package client

import (
	"sync"

	"meno/internal/api"
)

// Mirror is the client-side session state rebuilt from a snapshot and ordered events.
type Mirror struct {
	mu      sync.RWMutex
	State   MirrorState
	retired map[string]bool
}

// MirrorState contains view-ready session and live-run state.
type MirrorState struct {
	StartedCalls                 map[string]bool
	RetryMax                     int
	LastUsage, TotalUsage        api.UsageData
	Session                      api.SessionItem
	Cwd, Model, Phase, RequestID string
	Running                      bool
	Seq                          uint64
	Turns                        []api.Turn
	Live                         api.LiveState
	Ask                          *api.AskForm
	Retry                        *api.RetryState
	Metrics                      api.Metrics
	Queued                       []string
	Provider, Effort             string
	Spent                        float64
	Context                      int
	Unpriced                     bool
	Tasks                        []api.Task
	NeedsResync                  bool
	Version                      api.SessionVersion
	LastError                    string
}

// Load accepts a consistent snapshot, rejecting stale and retired boots.
func (m *Mirror) Load(s api.Snapshot) { m.LoadUpdate(s) }

func (m *Mirror) LoadUpdate(s api.Snapshot) (ProjectionUpdate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.Version.Revision == 0 {
		s.Version.Revision = s.Seq
	}
	old := m.State.Version
	if m.retired[s.Version.BootID] || old.Newer(s.Version) {
		return ProjectionUpdate{}, false
	}
	if old.BootID != "" && old.BootID != s.Version.BootID {
		if m.retired == nil {
			m.retired = map[string]bool{}
		}
		m.retired[old.BootID] = true
	}
	m.State = MirrorState{Session: s.Session, Cwd: s.Cwd, Model: s.Model, Phase: s.Phase, RequestID: s.RequestID, Running: s.Running, Seq: s.Seq, Turns: cloneTurns(s.Turns), Live: cloneLiveState(s.Live), Ask: cloneAsk(s.Ask), Retry: cloneRetry(s.Retry), Metrics: s.Metrics, Queued: append([]string(nil), s.Queued...), Provider: s.Provider, Effort: s.Effort, Spent: s.Spent, Context: s.Context, Unpriced: s.Unpriced, Tasks: cloneTasks(s.Tasks), Version: s.Version, LastError: s.LastError, StartedCalls: map[string]bool{}}
	if s.TotalUsage != nil {
		m.State.TotalUsage = *s.TotalUsage
	}
	if s.LastUsage != nil {
		m.State.LastUsage = *s.LastUsage
	}
	for _, c := range s.Live.Calls {
		m.State.StartedCalls[c.CallID] = c.Exit == nil
	}
	return m.projection(AllChanged, true, m.State.Turns, ProjectionEffects{}), true
}

// Snapshot returns a fully isolated copy for explicit readers, not token delivery.
func (m *Mirror) Snapshot() MirrorState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.State
	s.Turns, s.Live, s.Ask, s.Retry = cloneTurns(s.Turns), cloneLiveState(s.Live), cloneAsk(s.Ask), cloneRetry(s.Retry)
	s.Tasks, s.Queued, s.StartedCalls = cloneTasks(s.Tasks), append([]string(nil), s.Queued...), cloneStarted(s.StartedCalls)
	return s
}
func (m *Mirror) NeedsResync() bool { m.mu.RLock(); defer m.mu.RUnlock(); return m.State.NeedsResync }

func hasTurn(turns []api.Turn, id string) bool {
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
func cloneLiveState(l api.LiveState) api.LiveState {
	l.Calls = append([]api.CallState(nil), l.Calls...)
	for i := range l.Calls {
		if l.Calls[i].Exit != nil {
			n := *l.Calls[i].Exit
			l.Calls[i].Exit = &n
		}
	}
	return l
}
