package llm

import (
	"fmt"
	"sort"
	"strings"
)

// ModelSettings is what the user says about one model of a provider. It wins
// over the model catalog, field by field; a field left out keeps the catalog
// value. Prices are USD per million tokens.
type ModelSettings struct {
	Context    int      `json:"context,omitempty"`
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
	Vision     *bool    `json:"vision,omitempty"`
	// Efforts are the effort levels the model accepts, in order. Empty means
	// the standard levels.
	Efforts []string `json:"efforts,omitempty"`
}

// ValidateModels checks the per-model settings of a provider.
func ValidateModels(ms map[string]ModelSettings) error {
	names := make([]string, 0, len(ms))
	for k := range ms {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		m := ms[name]
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("models: a model id must not be empty")
		}
		if m.Context < 0 {
			return fmt.Errorf("models.%s: context must not be negative", name)
		}
		for label, p := range map[string]*float64{"input": m.Input, "output": m.Output, "cache_read": m.CacheRead, "cache_write": m.CacheWrite} {
			if p != nil && *p < 0 {
				return fmt.Errorf("models.%s: %s must not be negative", name, label)
			}
		}
		seen := map[string]bool{}
		for _, e := range m.Efforts {
			if e == "" || strings.ContainsAny(e, " \t\n") || seen[e] {
				return fmt.Errorf("models.%s: efforts must be unique words, got %q", name, e)
			}
			seen[e] = true
		}
	}
	return nil
}

// Lookup is LookupMetadata with the settings of this provider applied on top.
func (p Provider) Lookup(catalog map[string]ModelInfo, model string) (ModelInfo, bool) {
	info, ok := LookupMetadata(catalog, p, model)
	s, has := p.Models[model]
	if !has {
		return info, ok
	}
	if !ok {
		info = ModelInfo{MissingPricing: true}
	}
	if s.Context > 0 {
		info.Context = s.Context
	}
	if s.Input != nil {
		info.Input = *s.Input
	}
	if s.Output != nil {
		info.Output = *s.Output
	}
	if s.Input != nil && s.Output != nil {
		info.MissingPricing = false
	}
	if s.CacheRead != nil {
		info.CacheRead = s.CacheRead
	}
	if s.CacheWrite != nil {
		info.CacheWrite = s.CacheWrite
	}
	if s.Vision != nil {
		info.Vision = s.Vision
	}
	return info, true
}

// EffortsFor returns the effort levels of a model: its own list, or the
// standard levels given.
func (p Provider) EffortsFor(model string, standard []string) []string {
	if e := p.Models[model].Efforts; len(e) > 0 {
		return append([]string(nil), e...)
	}
	return standard
}

// ClampEffort returns cur when the model accepts it. Otherwise it returns
// "medium" or "high" if the model accepts one of them, else its first level.
func ClampEffort(allowed []string, cur string) string {
	if len(allowed) == 0 {
		return cur
	}
	for _, want := range []string{cur, "medium", "high"} {
		for _, a := range allowed {
			if a == want {
				return want
			}
		}
	}
	return allowed[0]
}
