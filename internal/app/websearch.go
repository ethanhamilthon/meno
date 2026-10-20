package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/websearch"
)

// webSearchHost searches the web for the TUI. The order is fixed: the
// provider's own search, then the backend chosen in settings, then the keyless
// DuckDuckGo fallback.
type webSearchHost struct {
	a   *App
	sid string
}

func (h webSearchHost) Search(ctx context.Context, query string, opts map[string]any) (any, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("websearch: the query is empty")
	}
	count := 5
	switch n := opts["count"].(type) {
	case int64:
		count = int(n)
	case float64:
		count = int(n)
	}
	count = min(max(count, 1), 10)

	var st config.Settings
	var providers []llm.Provider
	var provider, model string
	h.a.onUI(func() {
		st, providers = h.a.st.Settings, append([]llm.Provider(nil), h.a.st.Providers...)
		if rt := h.a.rts[h.sid]; rt != nil {
			provider, model = rt.Session.Provider, rt.Session.Model
		}
	})
	backend := st.WebSearch
	if backend == "" {
		backend = "auto"
	}
	if backend == "off" {
		return nil, errors.New("websearch is off (settings → Web → search)")
	}
	var errs []string
	note := func(name string, err error) {
		errs = append(errs, name+": "+err.Error())
	}

	// 1. The provider's native search. It costs one model call and knows its
	// own sources; chat completions providers have none.
	if p, ok := (llm.Provider{Name: provider}).Find(providers); ok && p.Protocol != llm.ProtoChat {
		res, err := llm.NativeWebSearch(ctx, p, model, query, count)
		if err == nil && (res.Answer != "" || len(res.Results) > 0) {
			return searchView(query, "native", res.Answer, nativeResults(res.Results)), nil
		}
		if err != nil {
			note("native", err)
		}
	}

	// 2. The paid backend from settings. Auto tries both with the same key,
	// so a Tavily key also works when the user left the choice on auto.
	paid := func(b websearch.Backend) (any, bool) {
		if st.WebSearchKey == "" {
			if backend == string(b) {
				note(string(b), errors.New("no search key (settings → Web → search key)"))
			}
			return nil, false
		}
		ans, err := websearch.Search(ctx, b, st.WebSearchKey, query, count)
		if err != nil {
			note(string(b), err)
			return nil, false
		}
		v, ok := answerView(query, string(b), ans)
		if !ok {
			note(string(b), errors.New("no results"))
		}
		return v, ok
	}
	if backend == "brave" || backend == "auto" {
		if v, ok := paid(websearch.Brave); ok {
			return v, nil
		}
	}
	if backend == "tavily" || backend == "auto" {
		if v, ok := paid(websearch.Tavily); ok {
			return v, nil
		}
	}

	// 3. DuckDuckGo needs no key: the last resort.
	ans, err := websearch.Search(ctx, websearch.DuckDuckGo, "", query, count)
	if err == nil {
		if v, ok := answerView(query, "duckduckgo", ans); ok {
			return v, nil
		}
		note("duckduckgo", errors.New("no results"))
	} else {
		note("duckduckgo", err)
	}
	return nil, fmt.Errorf("websearch failed: %s", strings.Join(errs, "; "))
}

// answerView renders a websearch.Answer for JS.
func answerView(query string, backend string, ans websearch.Answer) (any, bool) {
	if len(ans.Results) == 0 && ans.Answer == "" {
		return nil, false
	}
	rs := make([]any, len(ans.Results))
	for i, r := range ans.Results {
		rs[i] = map[string]any{"title": r.Title, "url": r.URL, "snippet": r.Snippet}
	}
	return searchView(query, backend, ans.Answer, rs), true
}

func nativeResults(in []llm.WebSearchResult) []any {
	out := make([]any, len(in))
	for i, r := range in {
		out[i] = map[string]any{"title": r.Title, "url": r.URL, "snippet": r.Snippet}
	}
	return out
}

func searchView(query, backend, answer string, results []any) map[string]any {
	out := map[string]any{"query": query, "backend": backend, "results": results}
	if answer != "" {
		out["answer"] = answer
	}
	return out
}
