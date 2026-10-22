package engine

import (
	"context"
	"meno/internal/api"
)

func (e *Engine) ApplyClientID(ctx context.Context, key string, fn func() (any, error)) (any, error) {
	if key == "" {
		return nil, dapi(api.CodeBadRequest, "client_id is required")
	}
	e.mu.Lock()
	if r, ok := e.ids[key]; ok {
		e.mu.Unlock()
		return r.value, r.err
	}
	e.mu.Unlock()
	v, err := fn()
	e.mu.Lock()
	if prior, ok := e.ids[key]; ok {
		v, err = prior.value, prior.err
	} else {
		e.ids[key] = idResult{v, err}
	}
	e.mu.Unlock()
	return v, err
}
