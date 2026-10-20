package engine

import (
	"context"

	"meno/internal/agent"
	"meno/internal/proto"
)

// usageData is what a client needs to count one answered request: tokens, the
// cache split and the price when the model has one.
func (e *Engine) usageData(id string, ev agent.Ev) proto.UsageData {
	u := ev.Usage
	d := proto.UsageData{InputTokens: int64(u.In), OutputTokens: int64(u.Out), Cached: int64(u.Cached), CacheWrite: int64(u.CacheWrite),
		CacheKnown: u.CacheReported, Provider: ev.Provider.Name, Model: ev.Model}
	s, err := e.load(context.Background(), id)
	if err != nil {
		return d
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := e.provider(s.session.Provider); ok {
		if mi, ok := p.Lookup(e.catalogView(), s.session.Model); ok && mi.CanPrice(u) {
			d.Cost, d.Priced = mi.Cost(u), true
		}
	}
	return d
}
