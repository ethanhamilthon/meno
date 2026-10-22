package engine

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/content"
	"meno/internal/llm"
	"meno/internal/safe"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) Message(ctx context.Context, id string, q api.MessageRequest) (api.MessageResponse, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return api.MessageResponse{}, notFound(err)
	}
	if strings.TrimSpace(q.Text) == "" && len(q.Attachments) == 0 {
		return api.MessageResponse{}, dapi(api.CodeBadRequest, "message is empty")
	}
	// Read the current settings before the session lock; a failed read must not
	// start a run with stale or empty defaults.
	reloadErr := e.reloadConfig()
	in := e.selectionInput()
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	if s.compacting {
		s.mu.Unlock()
		return api.MessageResponse{}, dapi(api.CodeConflict, "session is being compacted")
	}
	if s.ask != nil {
		s.mu.Unlock()
		return api.MessageResponse{}, dapi(api.CodeConflict, "session is waiting for an ask answer")
	}
	if s.running {
		att, perr := e.prepareAttachments(q.Attachments)
		if perr != nil {
			s.mu.Unlock()
			return api.MessageResponse{}, perr
		}
		// The queue is the one path into a running turn: the runner takes the
		// entry at its next step, so an image makes the model answer again.
		s.queue.Push(agent.Message{Text: q.Text, Attachments: att})
		rid := s.requestID
		e.publishQueue(s, api.InjectedData{Text: q.Text, By: "user"})
		s.mu.Unlock()
		return api.MessageResponse{RequestID: rid, Queued: true}, nil
	}
	if reloadErr != nil {
		s.mu.Unlock()
		return api.MessageResponse{}, dapi(api.CodeInternal, reloadErr.Error())
	}
	repaired, err := resolveSelection(s, in)
	if err != nil {
		s.mu.Unlock()
		return api.MessageResponse{}, err
	}
	untitled := s.session.Title == "" || s.session.Title == "untitled"
	if err = e.appendMessage(s, q); err != nil {
		s.mu.Unlock()
		return api.MessageResponse{}, err
	}
	rid := agent.NewID()
	s.requestID = rid
	s.origin = q.Origin
	s.lastErr = "" // the outcome of the previous run no longer applies
	s.running = true
	s.phase = "preparing"
	s.metrics = api.Metrics{}
	s.live = api.LiveState{Calls: []api.CallState{}}
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	_ = e.save(s)
	named := untitled && s.session.Title != "" && s.session.Title != "untitled"
	turns := s.session.Snapshot()
	s.published = len(turns)
	s.visibleTurns = wireTurns(turns)
	s.mu.Unlock()
	e.publish(id, api.EventTurnAdded, api.TurnAddedData{Turn: turnWire(turns[len(turns)-1])})
	e.publish(id, api.EventStatus, api.StatusData{Running: true, Phase: "preparing", RequestID: rid, Origin: q.Origin})
	if repaired {
		e.publishSessionChanged(s, "provider", "model", "effort")
	}
	if named {
		// The first message names the session; windows show it now, not at the end of the run.
		e.publishSessionChanged(s, "name")
	}
	e.launch(runCtx, s, q.Text, false)
	return api.MessageResponse{RequestID: rid}, nil
}

func (e *Engine) appendMessage(s *runState, q api.MessageRequest) error {
	att, err := e.prepareAttachments(q.Attachments)
	if err != nil {
		return err
	}
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: q.Text, Attachments: att, Time: time.Now().UTC()})
	return nil
}

// prepareAttachments decodes, validates and stores the images of a message.
func (e *Engine) prepareAttachments(in []api.Attachment) ([]agent.Attachment, error) {
	att := make([]agent.Attachment, 0, len(in))
	for i, a := range in {
		if a.Data == "" && a.Path == "" {
			return nil, dapi(api.CodeBadRequest, "attachment requires data or path")
		}
		var raw []byte
		var err error
		if a.Data != "" {
			raw, err = base64.StdEncoding.DecodeString(a.Data)
		} else {
			raw, err = os.ReadFile(a.Path)
		}
		if err != nil {
			return nil, dapi(api.CodeBadRequest, err.Error())
		}
		img, err := content.Prepare(raw)
		if err != nil {
			return nil, dapi(api.CodeBadRequest, err.Error())
		}
		path := filepath.Join(e.paths.File("attachments"), img.SHA+content.Ext(img.Mime))
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err = os.WriteFile(path, img.Data, 0600); err != nil {
				return nil, err
			}
		}
		att = append(att, agent.Attachment{N: i + 1, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H})
	}
	return att, nil
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
	effort := llm.ClampEffort(p.EffortsFor(model, []string{"low", "medium", "high", "max"}), current)
	s.mu.Lock()
	if s.session.Provider == s.pinProvider && s.session.Model == s.pinModel {
		s.session.Provider, s.session.Model, s.session.Effort = p.Name, model, effort
	}
	s.mu.Unlock()
	system, diff, ps := e.prepare(ctx, s, effort, s.prompt.system == "")
	if diff != "" {
		turns := s.session.Snapshot()
		if len(turns) > 0 {
			s.session.SetSystem(len(turns)-1, diff)
		}
	}
	s.prompt = ps
	spec := e.toolSpecFor(s)
	tool := e.newBashRunner(s)
	auto := agent.AutoCompact{}
	if info, ok := p.Lookup(e.catalogView(), model); ok {
		auto = agent.AutoCompact{Window: info.Context, Percent: e.cfg().CompactPercent, Keep: e.cfg().CompactKeep, SummaryTokens: e.cfg().CompactSummaryTokens}
	}
	deps := agent.Deps{Client: e.client(p), Provider: p, Model: model, Effort: effort, System: system, Tool: tool, Thinking: e.cfg().Thinking != "off", ToolDesc: spec.desc, ToolName: spec.name, Queue: &s.queue, AutoCompact: auto, Events: e.eventSource(s.session.ID),
		Cost: func(u llm.Usage) (float64, bool) {
			mi, ok := p.Lookup(e.catalogView(), model)
			if !ok || !mi.CanPrice(u) {
				return 0, false
			}
			return mi.Cost(u), true
		},
		Emit: func(ev agent.Ev) { e.agentEvent(s, ev) }}
	var runErr error
	panicErr := safe.Call(func() { runErr = (&agent.Runner{D: deps}).Continue(ctx, s.session) })
	err := runErr
	if panicErr != nil {
		err = panicErr
	}
	if err == nil {
		err = ctx.Err()
	}
	e.finish(s, err)
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
	// Admission and the positive WaitGroup delta share Close's lock. Once closed
	// is set, no callback can add a run after Close starts waiting for zero.
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		s.running, s.phase = false, ""
		s.mu.Unlock()
		return
	}
	e.runs.Add(1)
	e.activeRuns++
	e.mu.Unlock()
	// The run keeps the model the session had when it started: a model picked
	// a moment later applies to the next request.
	s.mu.Lock()
	s.pinProvider, s.pinModel, s.pinEffort = s.session.Provider, s.session.Model, s.session.Effort
	s.mu.Unlock()
	safe.Go(func() {
		defer func() {
			e.mu.Lock()
			e.activeRuns--
			e.mu.Unlock()
			e.runs.Done()
		}()
		e.run(ctx, s, text, cont)
	}, func(err error) { e.finish(s, err) })
}
