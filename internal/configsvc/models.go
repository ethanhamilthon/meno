package configsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/pathsearch"
)

func (s *Service) modelsResource(ctx context.Context, method, rawQuery string) (any, error) {
	if method != "GET" {
		return nil, notFound("models only supports GET")
	}
	q := queryValues(rawQuery)
	only := q.Get("provider")
	refresh := q.Get("refresh") == "true"
	providers, err := s.providers()
	if err != nil {
		return nil, err
	}
	active := make([]llm.Provider, 0, len(providers))
	found := only == ""
	for _, p := range providers {
		if p.Disabled {
			continue
		}
		if only != "" && p.Name != only {
			continue
		}
		if p.Name == only {
			found = true
		}
		active = append(active, p)
	}
	if !found {
		return nil, badString(fmt.Sprintf("provider %q is not configured", only))
	}
	catalog, err := s.store.LoadCatalog()
	if err != nil {
		return nil, err
	}
	catalogUpdated := false
	if q.Get("catalog") == "refresh" {
		fetch := s.fetchMetadata
		if fetch == nil {
			fetch = llm.FetchMetadata
		}
		next, e := fetch(ctx)
		if e == nil {
			catalog = next
			_ = s.store.SaveCatalog(catalog)
			catalogUpdated = true
		}
	}
	fetch := s.fetchModels
	if fetch == nil {
		fetch = llm.FetchModels
	}
	out := api.ModelsResponse{Models: []api.ConfigModel{}, Errors: map[string]string{}, CatalogUpdated: catalogUpdated}
	for _, p := range active {
		known := llm.CatalogModels(catalog, p)
		s.mu.Lock()
		for _, cached := range s.knownModels[p.Name] {
			known = appendModel(known, cached)
		}
		errText := s.modelErrors[p.Name]
		s.mu.Unlock()
		stale := errText != ""
		if refresh {
			ms, e := fetch(ctx, p)
			s.mu.Lock()
			if e != nil {
				errText = e.Error()
				s.modelErrors[p.Name] = errText
				stale = true
			} else {
				known = appendCatalog(known, ms)
				s.knownModels[p.Name] = ms
				delete(s.modelErrors, p.Name)
				errText = ""
				stale = false
			}
			s.mu.Unlock()
		}
		if errText != "" {
			out.Errors[p.Name] = errText
		}
		for _, m := range known {
			info, ok := p.Lookup(catalog, m.ID)
			item := api.ConfigModel{Provider: p.Name, ID: m.ID, Efforts: p.EffortsFor(m.ID, config.Choices("effort")), Stale: stale, Source: "catalog"}
			if !ok {
				item.Source = "provider"
			}
			if info.Context > 0 {
				n := info.Context
				item.ContextWindow = &n
			}
			item.Images = info.Vision
			if !info.MissingPricing && ok {
				item.Prices = &api.ConfigModelPrices{Input: info.Input, Output: info.Output, CacheRead: info.CacheRead, CacheWrite: info.CacheWrite, PerMillionTokens: true}
			}
			out.Models = append(out.Models, item)
		}
	}
	sort.Slice(out.Models, func(i, j int) bool {
		if out.Models[i].Provider != out.Models[j].Provider {
			return out.Models[i].Provider < out.Models[j].Provider
		}
		return out.Models[i].ID < out.Models[j].ID
	})
	if len(out.Errors) == 0 {
		out.Errors = nil
	}
	return out, nil
}

func appendModel(models []llm.Model, item llm.Model) []llm.Model {
	for _, m := range models {
		if m.ID == item.ID {
			return models
		}
	}
	return append(models, item)
}
func appendCatalog(models, fresh []llm.Model) []llm.Model {
	for _, item := range fresh {
		models = appendModel(models, item)
	}
	return models
}

func (s *Service) searchResource(ctx context.Context, rawQuery string) (any, error) {
	q := queryValues(rawQuery)
	root, query := q.Get("root"), q.Get("q")
	if root == "" {
		root = s.paths.Cwd
	}
	if query == "" {
		query = q.Get("query")
	}
	base, fuzzy, _ := pathsearch.Resolve(root, s.paths.Home, query)
	if root != "" {
		base = root
	}
	limit := queryInt(q, "limit", 50)
	entries := pathsearch.Search(ctx, base, fuzzy, limit)
	items := make([]api.FileSearchItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, api.FileSearchItem{Path: e.Path, Score: e.Score, Directory: e.Dir})
	}
	return api.FileSearchResponse{Items: items}, nil
}
func (s *Service) debugResource(rawQuery string) (any, error) {
	if s.debug == nil {
		return api.DebugRequestsResponse{Requests: []json.RawMessage{}}, nil
	}
	session := queryValues(rawQuery).Get("session")
	return api.DebugRequestsResponse{Requests: s.debug(session)}, nil
}
