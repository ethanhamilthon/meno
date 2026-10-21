package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/mcp"
	"meno/internal/proto"
	"os"
	"path/filepath"
)

func (e *Engine) Snapshot(ctx context.Context, id string) (proto.Snapshot, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.Snapshot{}, notFound(err)
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	turns := s.session.Snapshot()
	out := proto.Snapshot{Session: item(s), AgentToken: s.token.value, Cwd: s.cwd, Model: s.session.Model, Running: s.running, Phase: s.phase, RequestID: s.requestID, TurnCount: len(turns), Live: s.live, Ask: askWire(s.ask), Retry: s.retry, Metrics: s.metrics, Seq: 0, Tasks: []proto.Task{},
		Spent: s.session.Metrics.Spent, Context: s.session.Metrics.Context, Unpriced: s.session.Metrics.Unpriced, Provider: s.session.Provider, Effort: s.session.Effort}
	if !s.running {
		out.LastError = s.lastErr
	}
	for _, m := range s.queue.Items() {
		out.Queued = append(out.Queued, m.Text)
	}
	s.mu.Unlock()
	out.Session.Completed, out.Session.ReadThrough = e.attention(id)
	if e.events != nil {
		out.Seq = e.events.Sequence(id)
		out.Version = e.version(out.Seq)
	}
	for _, t := range turns {
		out.Turns = append(out.Turns, turnWire(t))
	}
	tasks, _ := e.tasks.List(id, true)
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, taskWire(t))
	}
	out.TaskCount = len(out.Tasks)
	return out, nil
}
func (e *Engine) PatchSession(ctx context.Context, id string, q proto.PatchSessionRequest) (proto.SessionItem, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.SessionItem{}, notFound(err)
	}
	s.mu.Lock()
	if q.Name != nil {
		n, er := agent.NormalizeTitle(*q.Name)
		if er != nil {
			s.mu.Unlock()
			return proto.SessionItem{}, api(proto.CodeBadRequest, er.Error())
		}
		s.session.Title = n
	}
	if q.Cwd != nil {
		cwd, er := filepath.Abs(*q.Cwd)
		if er != nil {
			s.mu.Unlock()
			return proto.SessionItem{}, er
		}
		if st, er := os.Stat(cwd); er != nil || !st.IsDir() {
			s.mu.Unlock()
			return proto.SessionItem{}, api(proto.CodeBadRequest, "cwd must be an existing directory")
		}
		s.prevCwd = s.cwd
		s.cwd = cwd
		s.session.Cwd = cwd
	}
	if q.Provider != nil {
		s.session.Provider = *q.Provider
	}
	if q.Model != nil {
		s.session.Model = *q.Model
	}
	if q.Effort != nil {
		s.session.Effort = *q.Effort
	}
	if err = e.save(s); err != nil {
		s.mu.Unlock()
		return proto.SessionItem{}, err
	}
	result := item(s)
	s.mu.Unlock()
	e.publish(id, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"name", "cwd", "model", "effort"}})
	return result, nil
}
func (e *Engine) DeleteSession(ctx context.Context, id string) error {
	s, err := e.load(ctx, id)
	if err != nil {
		return notFound(err)
	}
	s.mu.Lock()
	if s.running || s.ask != nil {
		s.mu.Unlock()
		return api(proto.CodeConflict, "session has active work")
	}
	unsaved := !s.persisted
	s.mu.Unlock()
	d, err := e.store.DB()
	if err != nil {
		return err
	}
	_, err = d.Exec("DELETE FROM sessions WHERE id=?", id)
	if err == nil {
		e.mu.Lock()
		delete(e.sessions, id)
		e.mu.Unlock()
		err = (mcp.ResultStore{Dir: e.paths.File("mcp-results")}).DeleteSession(id)
		if unsaved {
			return err
		}
		e.announce(proto.EventSessionDeleted, id, "")
	}
	return err
}
