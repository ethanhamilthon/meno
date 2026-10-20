package engine

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/attach"
	"meno/internal/llm"
	"meno/internal/proto"
	"meno/internal/safe"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) Message(ctx context.Context, id string, q proto.MessageRequest) (proto.MessageResponse, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.MessageResponse{}, notFound(err)
	}
	if strings.TrimSpace(q.Text) == "" && len(q.Attachments) == 0 {
		return proto.MessageResponse{}, api(proto.CodeBadRequest, "message is empty")
	}
	// Read the current settings before the session lock; a failed read must not
	// start a run with stale or empty defaults.
	reloadErr := e.reloadConfig()
	in := e.selectionInput()
	s.mu.Lock()
	if s.ask != nil {
		s.mu.Unlock()
		return proto.MessageResponse{}, api(proto.CodeConflict, "session is waiting for an ask answer")
	}
	if s.running {
		if len(q.Attachments) > 0 {
			if err = e.appendMessage(s, q); err != nil {
				s.mu.Unlock()
				return proto.MessageResponse{}, err
			}
		} else {
			s.queue.Push(agent.Message{Text: q.Text})
		}
		rid := s.requestID
		e.publishQueue(s, proto.InjectedData{Text: q.Text, By: "user"})
		s.mu.Unlock()
		if len(q.Attachments) > 0 {
			s.mu.Lock()
			_ = e.save(s)
			turns := s.session.Snapshot()
			s.published = len(turns)
			s.mu.Unlock()
			e.publish(id, proto.EventTurnAdded, proto.TurnAddedData{Turn: turnWire(turns[len(turns)-1])})
		}
		return proto.MessageResponse{RequestID: rid, Queued: true}, nil
	}
	if reloadErr != nil {
		s.mu.Unlock()
		return proto.MessageResponse{}, api(proto.CodeInternal, reloadErr.Error())
	}
	repaired, err := resolveSelection(s, in)
	if err != nil {
		s.mu.Unlock()
		return proto.MessageResponse{}, err
	}
	if len(q.Modes) > 0 {
		oldModes := strings.Join(s.session.Modes, " ")
		v, er := validateModes(q.Modes)
		if er != nil {
			s.mu.Unlock()
			return proto.MessageResponse{}, er
		}
		s.session.Modes = v
		s.modes = compose(v)
		s.token.groups = groupsFor(v)
		if oldModes != strings.Join(v, " ") {
			s.token.value, _ = newToken()
		}
	}
	if err = e.appendMessage(s, q); err != nil {
		s.mu.Unlock()
		return proto.MessageResponse{}, err
	}
	rid := agent.NewID()
	s.requestID = rid
	s.running = true
	s.phase = "preparing"
	s.metrics = proto.Metrics{}
	s.live = proto.LiveState{Calls: []proto.CallState{}}
	if s.modes.Goal {
		s.goal = agent.NewGoal(s.modes.Rounds)
	} else {
		s.goal = nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	_ = e.save(s)
	turns := s.session.Snapshot()
	s.published = len(turns)
	s.mu.Unlock()
	e.publish(id, proto.EventTurnAdded, proto.TurnAddedData{Turn: turnWire(turns[len(turns)-1])})
	e.publish(id, proto.EventStatus, proto.StatusData{Running: true, Phase: "preparing"})
	if repaired {
		e.publish(id, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"provider", "model", "effort"}})
	}
	e.launch(runCtx, s, q.Text, false)
	return proto.MessageResponse{RequestID: rid}, nil
}

func (e *Engine) appendMessage(s *runState, q proto.MessageRequest) error {
	att := make([]agent.Attachment, 0, len(q.Attachments))
	for i, a := range q.Attachments {
		if a.Data == "" && a.Path == "" {
			return api(proto.CodeBadRequest, "attachment requires data or path")
		}
		var raw []byte
		var err error
		if a.Data != "" {
			raw, err = base64.StdEncoding.DecodeString(a.Data)
		} else {
			raw, err = os.ReadFile(a.Path)
		}
		if err != nil {
			return api(proto.CodeBadRequest, err.Error())
		}
		img, err := attach.Prepare(raw)
		if err != nil {
			return api(proto.CodeBadRequest, err.Error())
		}
		path := filepath.Join(e.paths.File("attachments"), img.SHA+attach.Ext(img.Mime))
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err = os.WriteFile(path, img.Data, 0600); err != nil {
				return err
			}
		}
		att = append(att, agent.Attachment{N: i + 1, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H})
	}
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: q.Text, Attachments: att, Mode: strings.Join(s.session.Modes, " "), Time: time.Now().UTC()})
	return nil
}

func (e *Engine) run(ctx context.Context, s *runState, text string, cont bool) {
	if err := e.reloadConfig(); err != nil {
		e.finish(s, err)
		return
	}
	s.mu.Lock()
	providerName := s.pinProvider
	s.mu.Unlock()
	p, ok := e.provider(providerName)
	if !ok {
		e.finish(s, fmt.Errorf("provider %q is not configured", providerName))
		return
	}
	s.mu.Lock()
	model, current := s.pinModel, s.pinEffort
	s.mu.Unlock()
	if model == "" {
		model = e.cfg().Model
	}
	effort, err := e.effort(ctx, s, text, p, model, current)
	if err != nil {
		effort = current
	}
	s.mu.Lock()
	if s.session.Provider == s.pinProvider && s.session.Model == s.pinModel {
		s.session.Provider, s.session.Model, s.session.Effort = p.Name, model, effort
	}
	s.mu.Unlock()
	system, diff, ps := e.prepare(ctx, s, effort, s.prompt.system == "")
	if !cont && s.modes.Text != "" {
		system += "\n\n" + s.modes.Text
	}
	if diff != "" {
		turns := s.session.Snapshot()
		if len(turns) > 0 {
			s.session.SetSystem(len(turns)-1, diff)
		}
	}
	s.prompt = ps
	spec := e.toolSpecFor(s)
	tool := e.newBashRunner(s, e.bashTimeout())
	auto := agent.AutoCompact{}
	if info, ok := p.Lookup(e.catalogView(), model); ok {
		auto = agent.AutoCompact{Window: info.Context, Percent: e.cfg().CompactPercent, Keep: e.cfg().CompactKeep, SummaryTokens: e.cfg().CompactSummaryTokens}
	}
	deps := agent.Deps{Client: e.client(p), Provider: p, Model: model, Effort: effort, System: system, Tool: tool, Thinking: e.cfg().Thinking != "off", ModelTimeout: spec.modelTimeout, ToolDesc: spec.desc, ToolName: spec.name, DefaultTimeout: e.bashTimeout(), Goal: s.goal, Queue: &s.queue, AutoCompact: auto, Events: e.eventSource(s.session.ID),
		Cost: func(u llm.Usage) (float64, bool) {
			mi, ok := p.Lookup(e.catalogView(), model)
			if !ok || !mi.CanPrice(u) {
				return 0, false
			}
			return mi.Cost(u), true
		},
		Emit: func(ev agent.Ev) { e.agentEvent(s, ev) }}
	err = safe.Call(func() { err = (&agent.Runner{D: deps}).Continue(ctx, s.session) })
	if err == nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		s.mu.Lock()
		rid := s.requestID
		s.mu.Unlock()
		e.tasks.CancelOwner(s.session.ID, rid)
	}
	e.finish(s, err)
}

func (e *Engine) finish(s *runState, err error) {
	id := s.session.ID
	s.mu.Lock()
	s.running = false
	s.phase = ""
	s.cancel = nil
	s.retry = nil
	s.live = proto.LiveState{Calls: []proto.CallState{}}
	if err != nil && !errors.Is(err, context.Canceled) {
		s.lastErr = err.Error()
	}
	saveErr := e.save(s)
	queued := s.queue.Take()
	requestID, metrics := s.requestID, s.metrics
	s.mu.Unlock()
	if saveErr != nil {
		e.publish(id, proto.EventError, proto.ErrorData{Message: saveErr.Error()})
	}
	e.publish(id, proto.EventStatus, proto.StatusData{Running: false})
	if err != nil && errors.Is(err, context.Canceled) {
		e.publish(id, proto.EventAborted, map[string]any{})
	} else if err != nil {
		e.publish(id, proto.EventError, proto.ErrorData{Message: err.Error()})
	} else {
		e.publish(id, proto.EventDone, proto.DoneData{RequestID: requestID, Metrics: metrics})
	}
	if len(queued) > 0 && err == nil {
		s.mu.Lock()
		msg := agent.Combine(queued)
		s.session.Append(agent.Turn{Role: llm.RoleUser, Text: msg.Text, Attachments: msg.Attachments, Time: time.Now().UTC()})
		_ = e.save(s)
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.running = true
		s.phase = "preparing"
		s.requestID = agent.NewID()
		s.mu.Unlock()
		e.launch(ctx, s, msg.Text, false)
	}
}
func (e *Engine) HasWork(context.Context) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range e.sessions {
		s.mu.Lock()
		busy := s.running || s.ask != nil
		s.mu.Unlock()
		if busy {
			return true
		}
	}
	return false
}

func (e *Engine) launch(ctx context.Context, s *runState, text string, cont bool) {
	// The run keeps the model the session had when it started: a model picked
	// a moment later applies to the next request.
	s.mu.Lock()
	s.pinProvider, s.pinModel, s.pinEffort = s.session.Provider, s.session.Model, s.session.Effort
	s.mu.Unlock()
	e.runs.Add(1)
	safe.Go(func() { defer e.runs.Done(); e.run(ctx, s, text, cont) }, func(err error) { e.finish(s, err) })
}
