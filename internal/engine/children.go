package engine

import (
	"context"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/proto"
	"meno/internal/tools"
	"strings"
	"sync"
	"time"
)

type childHost struct {
	e      *Engine
	parent *runState
	vm     *jsvm.VM
}
type childRun struct {
	mu     sync.Mutex
	rec    agent.ChildRecord
	sess   *agent.Session
	ctx    context.Context
	cancel context.CancelFunc
	wait   chan struct{}
	block  string
	vm     *jsvm.VM
}

func (h childHost) Spawn(call jsvm.TaskCall, spec map[string]any) (any, error) {
	task, _ := spec["task"].(string)
	if strings.TrimSpace(task) == "" {
		return nil, errors.New("agents.spawn: task is required")
	}
	h.parent.mu.Lock()
	parentID, owner := h.parent.session.ID, h.parent.requestID
	spawned := h.parent.spawned
	h.parent.spawned++
	h.parent.mu.Unlock()
	if spawned >= h.e.settings.SubagentMaxSpawn {
		return nil, errors.New("agents.spawn: subagent limit reached")
	}
	cwd := call.Cwd
	if v, ok := spec["cwd"].(string); ok && v != "" {
		cwd = v
	}
	pr, model, effort := h.parent.session.Provider, h.parent.session.Model, h.parent.session.Effort
	if v, ok := spec["provider"].(string); ok && v != "" {
		pr = v
	}
	if v, ok := spec["model"].(string); ok && v != "" {
		model = v
	}
	if v, ok := spec["effort"].(string); ok && v != "" {
		effort = v
	}
	_, ok := h.e.provider(pr)
	if !ok {
		return nil, fmt.Errorf("agents.spawn: provider %q is not configured", pr)
	}
	if model == "" {
		return nil, errors.New("agents.spawn: model is required")
	}
	recList, err := h.e.children.List(parentID, false)
	if err != nil {
		return nil, err
	}
	working := 0
	for _, r := range recList {
		if r.Working() {
			working++
		}
	}
	if working >= h.e.settings.SubagentMaxSpawn {
		return nil, errors.New("agents.spawn: subagent limit reached")
	}
	sess := agent.NewSession(cwd, model)
	sess.Kind, sess.ParentID, sess.Provider, sess.Effort = agent.KindChild, parentID, pr, effort
	sess.AttachDir = h.e.paths.File("attachments")
	childState := &runState{calls: map[string]*tools.Call{}, session: sess, cwd: cwd, env: cloneMap(h.parent.env), changed: time.Now(), token: tokenState{groups: groupsFor(nil)}}
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	childState.token.value = tok
	childState.modes = compose(nil)
	rec := agent.ChildRecord{ID: sess.ID, Parent: parentID, Owner: owner, Task: task, Status: agent.ChildStarting, Gen: 1, Created: time.Now().UTC()}
	if err = agent.SaveSession(h.e.home, sess); err != nil {
		return nil, err
	}
	if err = h.e.children.Save(rec); err != nil {
		return nil, err
	}
	sess.Append(agent.Turn{Role: llm.RoleUser, Text: task, Time: time.Now().UTC()})
	_ = h.e.save(childState)
	h.e.mu.Lock()
	h.e.sessions[sess.ID] = childState
	h.e.mu.Unlock()
	runCtx := context.Background()
	safeGo := func() {
		runCtx2, cancel2 := context.WithCancel(runCtx)
		childState.mu.Lock()
		childState.cancel = cancel2
		childState.running = true
		childState.requestID = agent.NewID()
		childState.phase = "preparing"
		childState.mu.Unlock()
		h.e.safeRun(runCtx2, childState, task, false)
	}
	go safeGo()
	h.e.publish(parentID, proto.EventAgentsChanged, []string{sess.ID})
	return map[string]any{"id": sess.ID}, nil
}
func (h childHost) List(jsvm.TaskCall) (any, error) {
	rs, err := h.e.children.List(h.parent.session.ID, false)
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, r := range rs {
		out = append(out, childView(r))
	}
	return out, nil
}
func (h childHost) Status(_ jsvm.TaskCall, id string) (any, error) {
	r, err := h.e.children.Get(h.parent.session.ID, id)
	if err != nil {
		return nil, err
	}
	return childView(r), nil
}
func (h childHost) Send(_ jsvm.TaskCall, id, text string) (any, error) {
	r, err := h.e.children.Get(h.parent.session.ID, id)
	if err != nil {
		return nil, err
	}
	s, err := h.e.load(context.Background(), id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.running {
		s.queue.Push(agent.Message{Text: text})
		s.mu.Unlock()
		return map[string]any{"id": id, "queued": true}, nil
	}
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: text, Time: time.Now().UTC()})
	s.requestID = agent.NewID()
	s.running = true
	s.phase = "preparing"
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	h.e.safeRun(ctx, s, text, true)
	return map[string]any{"id": r.ID, "queued": false}, nil
}
func (h childHost) Wait(c jsvm.TaskCall, id string, d time.Duration) (any, error) {
	deadline := time.Time{}
	if d > 0 {
		deadline = time.Now().Add(d)
	}
	for {
		rec, err := h.e.children.Get(h.parent.session.ID, id)
		if err != nil {
			return nil, err
		}
		if !rec.Working() {
			return childView(rec), nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return map[string]any{"id": id, "waitTimedOut": true}, nil
		}
		select {
		case <-c.Ctx.Done():
			return nil, c.Ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func (h childHost) Result(_ jsvm.TaskCall, id string) (any, error) {
	r, err := h.e.children.Get(h.parent.session.ID, id)
	if err != nil {
		return nil, err
	}
	return childView(r), nil
}
func (h childHost) Cancel(_ jsvm.TaskCall, id string) (any, error) {
	r, err := h.e.children.Get(h.parent.session.ID, id)
	if err != nil {
		return nil, err
	}
	s, err := h.e.load(context.Background(), id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	r.Status = agent.ChildCancelled
	r.Ended = time.Now().UTC()
	_ = h.e.children.Save(r)
	h.e.publish(h.parent.session.ID, proto.EventAgentsChanged, []string{id})
	return childView(r), nil
}
func (h childHost) Report(_ jsvm.TaskCall, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("agents.report: text is empty")
	}
	h.e.publish(h.parent.session.ParentID, proto.EventAgentsChanged, map[string]any{"id": h.parent.session.ID, "report": text})
	return nil
}
func (h childHost) Block(_ jsvm.TaskCall, reason string) error {
	h.parent.mu.Lock()
	h.parent.lastErr = "blocked: " + reason
	h.parent.mu.Unlock()
	return nil
}
func childView(r agent.ChildRecord) map[string]any {
	var result any
	if r.Result != nil {
		result = *r.Result
	}
	return map[string]any{"id": r.ID, "status": r.Status, "task": r.Task, "generation": r.Gen, "result": result, "error": r.Error, "blockReason": r.BlockReason}
}
func (e *Engine) safeRun(ctx context.Context, s *runState, text string, cont bool) {
	e.runs.Add(1)
	go func() {
		defer e.runs.Done()
		defer func() {
			if v := recover(); v != nil {
				e.finish(s, fmt.Errorf("internal error: %v", v))
			}
		}()
		e.run(ctx, s, text, cont)
	}()
}
