package llm

import (
	"context"
	"fmt"
	"io"
	"meno/internal/safe"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

// ModelInfo costs are USD per million tokens; absence is distinct from free.
type ModelInfo struct {
	Context        int
	Input, Output  float64
	CacheRead      *float64
	CacheWrite     *float64
	MissingPricing bool // Catalog entry has context/identity but no complete token prices.
	// Vision tells whether the model accepts images; nil when the catalog does not say.
	Vision *bool `json:",omitempty"`
}

func (m ModelInfo) CanPrice(u Usage) bool {
	return !m.MissingPricing && (u.CacheWrite <= 0 || m.CacheWrite != nil)
}

func (m ModelInfo) Cost(u Usage) float64 {
	cached := min(max(0, u.Cached), max(0, u.In))
	read := m.Input
	if m.CacheRead != nil {
		read = *m.CacheRead
	}
	written := min(max(0, u.CacheWrite), max(0, u.In-cached))
	write := m.Input
	if m.CacheWrite != nil {
		write = *m.CacheWrite
	}
	return (float64(max(0, u.In-cached-written))*m.Input + float64(cached)*read + float64(written)*write + float64(max(0, u.Out))*m.Output) / 1e6
}

// FetchMetadata combines public model catalogs. Either source can serve as an
// offline-cache refresh when the other is unavailable; OpenRouter owns its prices.
func FetchMetadata(ctx context.Context) (map[string]ModelInfo, error) {
	// MENO_NO_METADATA keeps tests and air-gapped setups off the network.
	if os.Getenv("MENO_NO_METADATA") != "" {
		return map[string]ModelInfo{}, nil
	}
	type result struct {
		models map[string]ModelInfo
		err    error
		router bool
	}
	results := make(chan result, 2)
	for _, router := range []bool{false, true} {
		go func(router bool) {
			defer safe.Recover(nil)
			endpoint := "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
			decode := decodeLiteLLM
			if router {
				endpoint = "https://openrouter.ai/api/v1/models"
				decode = decodeOpenRouter
			}
			models, err := fetchCatalog(ctx, endpoint, decode)
			results <- result{models, err, router}
		}(router)
	}
	out := map[string]ModelInfo{}
	var lastErr error
	var routerModels map[string]ModelInfo
	for range 2 {
		r := <-results
		if r.err != nil {
			lastErr = r.err
			continue
		}
		if r.router {
			routerModels = r.models
		} else {
			out = r.models
		}
	}
	for id, m := range routerModels {
		if old, ok := out[id]; ok {
			if m.Context == 0 {
				m.Context = old.Context
			}
			if m.CacheRead == nil {
				m.CacheRead = old.CacheRead
			}
			if m.CacheWrite == nil {
				m.CacheWrite = old.CacheWrite
			}
			if m.Vision == nil {
				m.Vision = old.Vision
			}
			if m.MissingPricing && !old.MissingPricing {
				m.Input, m.Output, m.MissingPricing = old.Input, old.Output, false
			}
		}
		out[id] = m
	}
	if len(out) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("empty model catalogs")
		}
		return nil, lastErr
	}
	return out, nil
}

func fetchCatalog(ctx context.Context, endpoint string, decode func(io.Reader) (map[string]ModelInfo, error)) (map[string]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := modelsHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("model catalog: HTTP %d", resp.StatusCode)
	}
	return decode(io.LimitReader(resp.Body, 32<<20))
}

// MatchMetadata avoids guessing prices across providers for custom endpoints.
func MatchMetadata(catalog map[string]ModelInfo, p Provider, model string) (ModelInfo, bool) {
	provider := catalogProvider(p)
	m, ok := catalog[provider+"/"+model]
	return m, ok
}

// firstPartyOrder ranks catalog sources trusted for name-based lookups.
var firstPartyOrder = []string{"anthropic", "openai", "gemini", "deepseek", "xai", "mistral", "groq", "togetherai", "cerebras", "openrouter"}

func normModelID(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), ".", "-")
}

// LookupMetadata is MatchMetadata plus a name-based fallback for custom
// endpoints (proxies, gateways, local servers) whose host maps to no catalog
// provider. The model id is matched against first-party catalog entries
// (dots and dashes are interchangeable, vendor prefixes are ignored), so the
// result is an approximation of what the proxy really charges.
func LookupMetadata(catalog map[string]ModelInfo, p Provider, model string) (ModelInfo, bool) {
	if m, ok := MatchMetadata(catalog, p, model); ok {
		return m, true
	}
	if !strings.HasPrefix(catalogProvider(p), "custom:") || model == "" {
		return ModelInfo{}, false
	}
	want := normModelID(model)
	if i := strings.LastIndex(want, "/"); i >= 0 {
		want = want[i+1:]
	}
	best, bestRank := ModelInfo{}, len(firstPartyOrder)
	for id, info := range catalog {
		prov, rest, ok := strings.Cut(id, "/")
		if !ok {
			continue
		}
		rank := -1
		for i, name := range firstPartyOrder {
			if name == prov {
				rank = i
				break
			}
		}
		if rank < 0 || rank >= bestRank {
			continue
		}
		if i := strings.LastIndex(rest, "/"); i >= 0 {
			rest = rest[i+1:]
		}
		if normModelID(rest) == want && (!info.MissingPricing || bestRank == len(firstPartyOrder)) {
			best, bestRank = info, rank
		}
	}
	return best, bestRank < len(firstPartyOrder)
}

// CatalogModels maps catalog IDs to the IDs accepted by a configured endpoint.
func CatalogModels(catalog map[string]ModelInfo, p Provider) []Model {
	prefix := catalogProvider(p) + "/"
	var models []Model
	for id := range catalog {
		if strings.HasPrefix(id, prefix) {
			models = append(models, Model{ID: strings.TrimPrefix(id, prefix), Provider: p.Name})
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func catalogProvider(p Provider) string {
	u, _ := url.Parse(p.BaseURL)
	known := map[string]string{"api.openai.com": "openai", "openrouter.ai": "openrouter", "api.anthropic.com": "anthropic", "api.deepseek.com": "deepseek", "api.groq.com": "groq", "api.mistral.ai": "mistral", "api.x.ai": "xai", "api.together.xyz": "togetherai", "api.together.ai": "togetherai", "generativelanguage.googleapis.com": "gemini", "api.cerebras.ai": "cerebras"}
	if u != nil {
		if id, ok := known[strings.ToLower(u.Hostname())]; ok {
			return id
		}
	}
	// An explicit unknown endpoint must not inherit another provider's prices.
	if p.BaseURL != "" {
		return "custom:" + strings.ToLower(p.Name)
	}
	return strings.ToLower(p.Name)
}
