package llm

import (
	"encoding/json"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

func decodeOpenRouter(r io.Reader) (map[string]ModelInfo, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&response); err != nil {
		return nil, err
	}
	out := map[string]ModelInfo{}
	for _, raw := range response.Data {
		// Pricing carries non-price members (e.g. "overrides" is an array), so keep
		// values raw and only read the string-encoded prices. A malformed entry
		// must not discard the whole catalog.
		var m struct {
			ID      string                     `json:"id"`
			Context int                        `json:"context_length"`
			Pricing map[string]json.RawMessage `json:"pricing"`
			Arch    *struct {
				Input []string `json:"input_modalities"`
			} `json:"architecture"`
		}
		if json.Unmarshal(raw, &m) != nil || m.ID == "" {
			continue
		}
		price := func(key string) *float64 {
			var s string
			if json.Unmarshal(m.Pricing[key], &s) != nil {
				return nil
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil
			}
			v *= 1e6
			return &v
		}
		input, output := price("prompt"), price("completion")
		info := ModelInfo{Context: m.Context, CacheRead: price("input_cache_read"), CacheWrite: price("input_cache_write"), MissingPricing: input == nil || output == nil}
		if !info.MissingPricing {
			info.Input, info.Output = *input, *output
		}
		if m.Arch != nil && len(m.Arch.Input) > 0 {
			v := slices.Contains(m.Arch.Input, "image")
			info.Vision = &v
		}
		out["openrouter/"+m.ID] = info
	}
	return out, nil
}

func decodeLiteLLM(r io.Reader) (map[string]ModelInfo, error) {
	var catalog map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&catalog); err != nil {
		return nil, err
	}
	out := map[string]ModelInfo{}
	for id, raw := range catalog {
		// Entries such as "sample_spec" hold documentation strings in numeric
		// fields; skip any entry that does not decode instead of failing all.
		var m struct {
			Provider   string   `json:"litellm_provider"`
			Mode       string   `json:"mode"`
			Context    *float64 `json:"max_input_tokens"`
			MaxTokens  *float64 `json:"max_tokens"`
			Input      *float64 `json:"input_cost_per_token"`
			Output     *float64 `json:"output_cost_per_token"`
			CacheRead  *float64 `json:"cache_read_input_token_cost"`
			CacheWrite *float64 `json:"cache_creation_input_token_cost"`
			Vision     *bool    `json:"supports_vision"`
		}
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.Provider == "" || (m.Mode != "chat" && m.Mode != "completion" && m.Mode != "responses") {
			continue
		}
		provider := m.Provider
		switch provider {
		case "text-completion-openai":
			provider = "openai"
		case "together_ai":
			provider = "togetherai"
		}
		model := strings.TrimPrefix(id, m.Provider+"/")
		model = strings.TrimPrefix(model, provider+"/")
		scale := func(v *float64) *float64 {
			if v == nil || *v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
				return nil
			}
			n := *v * 1e6
			return &n
		}
		input, output := scale(m.Input), scale(m.Output)
		info := ModelInfo{CacheRead: scale(m.CacheRead), CacheWrite: scale(m.CacheWrite), MissingPricing: input == nil || output == nil, Vision: m.Vision}
		if m.Context != nil && *m.Context > 0 {
			info.Context = int(*m.Context)
		} else if m.MaxTokens != nil && *m.MaxTokens > 0 {
			info.Context = int(*m.MaxTokens)
		}
		if !info.MissingPricing {
			info.Input, info.Output = *input, *output
		}
		out[provider+"/"+model] = info
	}
	return out, nil
}
