package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/proto"
	"os"
	"path/filepath"
	"strings"
)

func (e *Engine) Snapshot(ctx context.Context, id string) (proto.Snapshot, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.Snapshot{}, notFound(err)
	}
	s.mu.Lock()
	turns := s.session.Snapshot()
	out := proto.Snapshot{Session: item(s), AgentToken: s.token.value, Cwd: s.cwd, Model: s.session.Model, Modes: append([]string(nil), s.session.Modes...), Running: s.running, Phase: s.phase, RequestID: s.requestID, TurnCount: len(turns), Live: s.live, Ask: askWire(s.ask), Retry: s.retry, Metrics: s.metrics, Seq: 0, Tasks: []proto.Task{}, Agents: []proto.Agent{}}
	s.mu.Unlock()
	if e.events != nil {
		out.Seq = e.events.Sequence(id)
	}
	for _, t := range turns {
		out.Turns = append(out.Turns, turnWire(t))
	}
	tasks, _ := e.tasks.List(id, true)
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, taskWire(t))
	}
	children, _ := e.children.List(id, false)
	for _, c := range children {
		out.Agents = append(out.Agents, childWire(c))
	}
	out.TaskCount = len(out.Tasks)
	out.AgentCount = len(out.Agents)
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
	if q.Model != nil {
		s.session.Model = *q.Model
	}
	if q.Effort != nil {
		s.session.Effort = *q.Effort
	}
	if q.Modes != nil {
		oldModes := strings.Join(s.session.Modes, " ")
		v, er := validateModes(*q.Modes)
		if er != nil {
			s.mu.Unlock()
			return proto.SessionItem{}, er
		}
		s.session.Modes = v
		if oldModes != strings.Join(v, " ") {
			s.token.value, _ = newToken()
		}
		s.token.groups = groupsFor(v)
	}
	if err = e.save(s); err != nil {
		s.mu.Unlock()
		return proto.SessionItem{}, err
	}
	result := item(s)
	s.mu.Unlock()
	e.publish(id, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"name", "cwd", "model", "effort", "modes"}})
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
	}
	return err
}
