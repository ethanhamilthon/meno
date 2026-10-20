package engine

import (
	"context"
	"meno/internal/llm"
	"meno/internal/routing"
)

func (e *Engine) effort(ctx context.Context, s *runState, text string, p llm.Provider, model, current string) (string, error) {
	if e.cfg().EffortRouter != "auto" {
		return llm.ClampEffort(p.EffortsFor(model, []string{"low", "medium", "high", "max"}), current), nil
	}
	name, routeModel := e.cfg().EffortRouterProvider, e.cfg().EffortRouterModel
	if name == "" || routeModel == "" {
		return current, nil
	}
	rp, ok := e.provider(name)
	if !ok {
		return current, nil
	}
	v, err := (routing.ModelRouter{Client: e.client(rp), Provider: rp, Model: routeModel}).Route(ctx, routing.Request{Context: routing.Context{Text: text, Modes: append([]string(nil), s.session.Modes...)}, Current: current, Allowed: p.EffortsFor(model, []string{"low", "medium", "high", "max"})})
	if err != nil {
		return current, err
	}
	if s.sticky == nil {
		s.sticky = &routing.Sticky{Current: current}
	}
	level, _ := s.sticky.Decide(v)
	return llm.ClampEffort(p.EffortsFor(model, []string{"low", "medium", "high", "max"}), level), nil
}
