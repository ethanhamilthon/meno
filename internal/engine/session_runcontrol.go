package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/proto"
)

func (e *Engine) Stop(ctx context.Context, id string, q proto.StopRequest) error {
	s, err := e.load(ctx, id)
	if err != nil {
		return notFound(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if q.RequestID != "" && s.requestID != q.RequestID {
		return api(proto.CodeConflict, "request id does not match active request")
	}
	if s.requestID != "" {
		s.cancelled[s.requestID] = true
		if e.events2 != nil {
			_ = e.events2.CancelOwner(id, s.requestID)
		}
		e.tasks.CancelOwner(id, s.requestID)
	}
	owner := s.requestID
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	e.cancelChildren(id, owner)
	s.mu.Lock()
	return nil
}
func (e *Engine) Compact(ctx context.Context, id string, q proto.CompactRequest) (proto.CompactResponse, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.CompactResponse{}, notFound(err)
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return proto.CompactResponse{}, api(proto.CodeConflict, "cannot compact while a run is active")
	}
	p, ok := e.provider(s.session.Provider)
	model, effort, cwd := s.session.Model, s.session.Effort, s.cwd
	s.mu.Unlock()
	if !ok {
		return proto.CompactResponse{}, api(proto.CodeBadRequest, "session provider is not configured")
	}
	system, _, _ := e.prepare(ctx, s, effort, false)
	before := s.session.Len()
	_, _, err = agent.CompactWith(ctx, e.client(p), p, model, s.session, agent.CompactOptions{Keep: q.Keep, SummaryTokens: e.cfg().CompactSummaryTokens, Effort: effort, ModelTimeout: e.cfg().BashModelTimeout(), System: system, ToolDesc: ""}, nil)
	if err != nil {
		return proto.CompactResponse{}, err
	}
	s.mu.Lock()
	s.cwd = cwd
	err = e.save(s)
	after := s.session.Len()
	summary := s.session.Summary
	s.mu.Unlock()
	if err != nil {
		return proto.CompactResponse{}, err
	}
	e.publish(id, proto.EventInjected, map[string]any{"kind": "compact"})
	return proto.CompactResponse{Removed: before - after, Summary: summary}, nil
}
