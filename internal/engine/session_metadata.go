package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/mcp"
	"os"
	"path/filepath"
)

func (e *Engine) Snapshot(ctx context.Context, id string) (api.Snapshot, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return api.Snapshot{}, notFound(err)
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	turns := append([]api.Turn(nil), s.visibleTurns...)
	if !s.running && !s.compacting {
		turns = wireTurns(s.session.Snapshot())
	}
	out := api.Snapshot{Session: item(s), AgentToken: s.token.value, Cwd: s.cwd, Model: s.session.Model, Running: s.running, Phase: s.phase, RequestID: s.requestID, Origin: s.origin, TurnCount: len(turns), Live: cloneLive(s.live), Ask: askWire(s.ask), Retry: cloneRetry(s.retry), Metrics: s.metrics, Seq: 0, Tasks: []api.Task{},
		Spent: s.session.Metrics.Spent, Context: s.session.Metrics.Context, Unpriced: s.session.Metrics.Unpriced, Provider: s.session.Provider, Effort: s.session.Effort}
	out.TotalUsage = usageProjection(s.session.Metrics.Usage)
	out.LastUsage = usageProjection(s.session.Metrics.LastUsage)
	if !s.running {
		out.LastError = s.lastErr
	}
	out.Queued = append([]string(nil), s.visibleQueue...)
	s.mu.Unlock()
	out.Session.Completed, out.Session.ReadThrough = e.attention(id)
	if e.events != nil {
		out.Seq = e.events.Sequence(id)
		out.Version = e.version(out.Seq)
	}
	out.Turns = turns
	tasks, _ := e.tasks.List(id, true)
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, taskWire(t))
	}
	out.TaskCount = len(out.Tasks)
	return out, nil
}
func (e *Engine) PatchSession(ctx context.Context, id string, q api.PatchSessionRequest) (api.SessionItem, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return api.SessionItem{}, notFound(err)
	}
	var pick *modelChoice
	if q.Select != nil {
		s.mu.Lock()
		cur := modelChoice{s.session.Provider, s.session.Model, s.session.Effort}
		busy := s.running
		s.mu.Unlock()
		if busy {
			return api.SessionItem{}, dapi(api.CodeConflict, "the model can be changed only while the session is idle")
		}
		p, er := e.pickModel(q.Select.Model, q.Select.Effort, cur)
		if er != nil {
			return api.SessionItem{}, er
		}
		pick = &p
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	if pick != nil {
		if s.running {
			s.mu.Unlock()
			return api.SessionItem{}, dapi(api.CodeConflict, "the model can be changed only while the session is idle")
		}
		s.session.Provider, s.session.Model, s.session.Effort = pick.provider, pick.model, pick.effort
	}
	if q.Name != nil {
		n, er := agent.NormalizeTitle(*q.Name)
		if er != nil {
			s.mu.Unlock()
			return api.SessionItem{}, dapi(api.CodeBadRequest, er.Error())
		}
		s.session.Title = n
	}
	if q.Cwd != nil {
		cwd, er := filepath.Abs(*q.Cwd)
		if er != nil {
			s.mu.Unlock()
			return api.SessionItem{}, er
		}
		if st, er := os.Stat(cwd); er != nil || !st.IsDir() {
			s.mu.Unlock()
			return api.SessionItem{}, dapi(api.CodeBadRequest, "cwd must be an existing directory")
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
		return api.SessionItem{}, err
	}
	s.mu.Unlock()
	result := e.sessionMetadata(s).Session
	e.publishSessionChanged(s, "name", "cwd", "model", "effort")
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
		return dapi(api.CodeConflict, "session has active work")
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
		e.announce(api.EventSessionDeleted, id, "")
	}
	return err
}
