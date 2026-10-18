package app

import (
	"context"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"sort"
	"sync"
	"time"
)

// modelsSource is what Models needs from the UI state, copied on the UI goroutine.
type modelsSource struct {
	providers []llm.Provider
	known     map[string][]llm.Model
	errs      map[string]string
	pricing   map[string]llm.ModelInfo
	efforts   []string
}

func (h sessionHost) snapshot() modelsSource {
	var src modelsSource
	h.a.onUI(func() {
		a := h.a
		src.providers = append(src.providers, a.st.Providers...)
		src.known = map[string][]llm.Model{}
		for _, p := range a.st.Providers {
			src.known[p.Name] = a.modelsOf(p)
		}
		src.errs = map[string]string{}
		for k, v := range a.st.ModelErr {
			src.errs[k] = v
		}
		src.pricing = a.st.Pricing
		src.efforts = config.Choices("effort")
	})
	return src
}

func (h sessionHost) Models(opts map[string]any) (any, error) {
	only, refresh := "", false
	for k, v := range opts {
		switch k {
		case "provider":
			s, ok := v.(string)
			if !ok {
				return nil, errors.New("provider must be a string")
			}
			only = s
		case "refresh":
			refresh, _ = v.(bool)
		default:
			return nil, fmt.Errorf("unknown option %q", k)
		}
	}
	src := h.snapshot()
	if only != "" {
		found := false
		for _, p := range src.providers {
			found = found || p.Name == only
		}
		if !found {
			return nil, fmt.Errorf("provider %q is not configured", only)
		}
	}
	type result struct {
		models []llm.Model
		err    string
		stale  bool
	}
	res := map[string]*result{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range src.providers {
		if only != "" && p.Name != only {
			continue
		}
		r := &result{models: src.known[p.Name], err: src.errs[p.Name]}
		res[p.Name] = r
		if !refresh {
			continue
		}
		wg.Add(1)
		go func(p llm.Provider, r *result) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ms, err := llm.FetchModels(ctx, p)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				r.err, r.stale = err.Error(), true // keep the older list, marked stale
				return
			}
			r.err, r.models = "", ms
			h.a.postEvent(modelsLoaded{provider: p.Name, models: ms})
		}(p, r)
	}
	wg.Wait()
	models := []map[string]any{}
	errs := map[string]string{}
	for _, p := range src.providers {
		r := res[p.Name]
		if r == nil {
			continue
		}
		if r.err != "" {
			errs[p.Name] = r.err
		}
		for _, m := range r.models {
			info, ok := llm.LookupMetadata(src.pricing, p, m.ID)
			entry := map[string]any{
				"provider": p.Name, "id": m.ID, "efforts": src.efforts,
				"contextWindow": nil, "images": nil, "prices": nil,
				"stale": r.stale, "source": "catalog",
			}
			if ok {
				entry["contextWindow"] = nilIf0(info.Context)
				if info.Vision != nil {
					entry["images"] = *info.Vision
				}
				if !info.MissingPricing {
					entry["prices"] = map[string]any{"input": info.Input, "output": info.Output, "cacheRead": info.CacheRead, "cacheWrite": info.CacheWrite, "perMillionTokens": true}
				}
			}
			models = append(models, entry)
		}
	}
	sort.SliceStable(models, func(i, j int) bool {
		if models[i]["provider"] != models[j]["provider"] {
			return models[i]["provider"].(string) < models[j]["provider"].(string)
		}
		return models[i]["id"].(string) < models[j]["id"].(string)
	})
	return toAny(map[string]any{"models": models, "errors": errs})
}

func nilIf0(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// jsState adapts the versioned store of a session to the state global. Every
// write records the history boundary of the step that is executing.
type jsState struct {
	kv   *agent.KV
	sess *agent.Session
}

func (s jsState) Set(k, j string) error              { return s.kv.Set(k, j, s.sess.ExecBoundary()) }
func (s jsState) Get(k string) (string, bool, error) { return s.kv.Get(k) }
func (s jsState) Delete(k string) (bool, error)      { return s.kv.Delete(k, s.sess.ExecBoundary()) }
func (s jsState) Entries() ([]jsvm.StateEntry, error) {
	es, err := s.kv.Entries()
	out := make([]jsvm.StateEntry, len(es))
	for i, e := range es {
		out[i] = jsvm.StateEntry{Key: e.Key, JSON: e.JSON}
	}
	return out, err
}
