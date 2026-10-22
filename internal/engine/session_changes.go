package engine

import (
	"context"
	"meno/internal/api"
	"meno/internal/llm"
)

// sessionMetadata requires commit, but never performs attention I/O under mu.
func (e *Engine) sessionMetadata(s *runState) *api.SessionMetadata {
	s.mu.Lock()
	m := &api.SessionMetadata{Session: item(s), Provider: s.session.Provider, Effort: s.session.Effort}
	s.mu.Unlock()
	m.Session.Completed, m.Session.ReadThrough = e.attention(m.Session.ID)
	return m
}

// publishSessionChanged requires commit so metadata and cursor are one boundary.
func (e *Engine) publishSessionChanged(s *runState, fields ...string) {
	e.publish(s.session.ID, api.EventSessionChanged, api.ChangedData{
		Fields: fields, Metadata: e.sessionMetadata(s),
	})
}

// publishTasksChanged queries after acquiring commit, not from a start/promote
// reply: a fast completion may already have published newer supervisor state.
// No caller may hold mu or commit. Supervisor callbacks run outside its lock.
func (e *Engine) publishTasksChanged(id string) {
	s, err := e.load(context.Background(), id)
	if err != nil {
		return
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	tasks, err := e.tasks.List(id, true)
	if err != nil {
		return
	}
	out := make([]api.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskWire(t))
	}
	e.publish(id, api.EventTasksChanged, api.TasksChangedData{Tasks: out})
}

func usageProjection(u llm.Usage) *api.UsageData {
	return &api.UsageData{InputTokens: int64(u.In), OutputTokens: int64(u.Out), Cached: int64(u.Cached), CacheWrite: int64(u.CacheWrite), CacheKnown: u.CacheReported}
}
