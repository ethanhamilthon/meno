package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
)

func (e *Engine) Fork(ctx context.Context, id string, q api.ForkRequest) (api.ForkResponse, error) {
	src, err := e.load(ctx, id)
	if err != nil {
		return api.ForkResponse{}, notFound(err)
	}
	all, err := agent.AllTurns(e.home, id)
	if err != nil {
		return api.ForkResponse{}, err
	}
	boundary := int64(q.Turn)
	if boundary > 0 && boundary <= int64(len(all)) {
		boundary = all[boundary-1].HID
	}
	if boundary == 0 && len(all) > 0 {
		boundary = all[len(all)-1].HID
	}
	child, warn, err := agent.ForkSession(e.home, id, boundary, "")
	if err != nil {
		return api.ForkResponse{}, dapi(api.CodeBadRequest, err.Error())
	}
	if warn != "" {
		_ = warn
	}
	s, err := e.load(ctx, child.ID)
	if err != nil {
		return api.ForkResponse{}, err
	}
	src.mu.Lock()
	s.env = cloneMap(src.env)
	src.mu.Unlock()
	e.announce(api.EventSessionCreated, child.ID, id)
	s.mu.Lock()
	it := item(s)
	s.mu.Unlock()
	return api.ForkResponse{Session: it}, nil
}
func (e *Engine) Turns(ctx context.Context, id string, q api.TurnsPageRequest) (api.TurnsPage, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return api.TurnsPage{}, notFound(err)
	}
	s.mu.Lock()
	all := s.session.Snapshot()
	s.mu.Unlock()
	from := max(0, min(q.From, len(all)))
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	end := min(len(all), from+limit)
	out := api.TurnsPage{From: from, Next: end, Total: len(all), Turns: []api.Turn{}}
	for _, t := range all[from:end] {
		out.Turns = append(out.Turns, turnWire(t))
	}
	return out, nil
}
