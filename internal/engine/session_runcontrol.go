package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
)

func (e *Engine) Stop(ctx context.Context, id string, q api.StopRequest) error {
	s, err := e.load(ctx, id)
	if err != nil {
		return notFound(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if q.RequestID != "" && s.requestID != q.RequestID {
		return dapi(api.CodeConflict, "request id does not match active request")
	}
	if s.requestID != "" {
		s.cancelled[s.requestID] = true
		if e.events2 != nil {
			_ = e.events2.CancelOwner(id, s.requestID)
		}
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}
func (e *Engine) Compact(ctx context.Context, id string, q api.CompactRequest) (api.CompactResponse, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return api.CompactResponse{}, notFound(err)
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return api.CompactResponse{}, dapi(api.CodeConflict, "cannot compact while a run is active")
	}
	if s.compacting {
		s.mu.Unlock()
		return api.CompactResponse{}, dapi(api.CodeConflict, "the session is already being compacted")
	}
	s.compacting = true
	defer func() { s.mu.Lock(); s.compacting = false; s.mu.Unlock() }()
	p, ok := e.provider(s.session.Provider)
	model, effort, cwd := s.session.Model, s.session.Effort, s.cwd
	s.mu.Unlock()
	if !ok {
		return api.CompactResponse{}, dapi(api.CodeBadRequest, "session provider is not configured")
	}
	system, _, _ := e.prepare(ctx, s, effort, false)
	before := s.session.Len()
	_, _, err = agent.CompactWith(ctx, e.client(p), p, model, s.session, agent.CompactOptions{Keep: q.Keep, SummaryTokens: e.cfg().CompactSummaryTokens, Effort: effort, System: system, ToolDesc: "", RecordChanged: func() { e.agentEvent(s, agent.Ev{Kind: agent.EvRequestRecord}) }}, nil)
	if err != nil {
		return api.CompactResponse{}, err
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	s.cwd = cwd
	err = e.save(s)
	after := s.session.Len()
	summary := s.session.Summary
	queued := queuedText(s)
	s.mu.Unlock()
	if err != nil {
		return api.CompactResponse{}, err
	}
	e.publishAgentTurns(id, s.session.Snapshot())
	e.publish(id, api.EventInjected, api.InjectedData{Kind: "compact", Queued: queued})
	return api.CompactResponse{Removed: before - after, Summary: summary}, nil
}
