package engine

import (
	"context"
	"errors"
	"strings"

	"meno/internal/api"
	"meno/internal/llm"
	"meno/internal/web"
)

// webResource serves `meno web search|fetch`. The network runs in the daemon
// under the web policy; the agent only sees the typed result.
func (e *Engine) webResource(ctx context.Context, parts []string, method string, data map[string]any) (any, error) {
	if method != "POST" || len(parts) != 1 {
		return nil, dapi(api.CodeNotFound, "resource not found")
	}
	// Read the settings now: web_search and web_allow_local may have changed
	// since the last run, and a stale policy must never be applied.
	if err := e.reloadConfig(); err != nil {
		return nil, dapi(api.CodeInternal, err.Error())
	}
	cfg := e.cfg()
	svc := &web.Service{Policy: web.Policy{AllowLocal: cfg.WebAllowLocal}}
	switch parts[0] {
	case "fetch":
		u := stringValue(data, "url")
		if u == "" {
			return nil, dapi(api.CodeBadRequest, "url is required")
		}
		page, err := svc.Fetch(ctx, u, web.FetchOptions{})
		if err != nil {
			return page, webError(err)
		}
		return page, nil
	case "search":
		q := stringValue(data, "query")
		if q == "" {
			return nil, dapi(api.CodeBadRequest, "query is required")
		}
		backend, err := e.searchBackend(cfg.WebSearch)
		if err != nil {
			return nil, err
		}
		svc.Backend = backend
		res, err := svc.Search(ctx, q, web.SearchOptions{})
		if err != nil {
			return nil, webError(err)
		}
		return res, nil
	}
	return nil, dapi(api.CodeNotFound, "resource not found")
}

func webError(err error) error {
	switch {
	case errors.Is(err, web.ErrBlocked):
		return dapi(api.CodeForbidden, err.Error())
	case errors.Is(err, web.ErrNotConfigured), errors.Is(err, llm.ErrSearchUnsupported):
		return dapi(api.CodeBadRequest, err.Error())
	}
	return dapi(api.CodeInternal, err.Error())
}

type providerSearch struct {
	e        *Engine
	provider llm.Provider
	model    string
}

func (p providerSearch) Search(ctx context.Context, query string) (web.SearchResult, error) {
	a, err := llm.Search(ctx, p.provider, p.model, query)
	if err != nil {
		return web.SearchResult{}, err
	}
	out := web.SearchResult{Answer: a.Text, Provider: p.provider.Name, Model: p.model}
	for _, s := range a.Sources {
		out.Sources = append(out.Sources, web.Source{Title: s.Title, URL: s.URL})
	}
	return out, nil
}

// searchBackend resolves the web_search setting. An empty or unknown setting is
// an explicit error: no provider is chosen for the user.
func (e *Engine) searchBackend(setting string) (web.SearchBackend, error) {
	name, model, ok := strings.Cut(setting, "/")
	if setting == "" || !ok || name == "" || model == "" {
		return nil, webError(web.ErrNotConfigured)
	}
	p, found := e.provider(name)
	if !found {
		return nil, dapi(api.CodeBadRequest, "web_search provider "+name+" is not configured")
	}
	return providerSearch{e: e, provider: p, model: model}, nil
}
