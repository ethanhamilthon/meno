package llm

import (
	"encoding/json"
	"io"
)

func decodeMetadata(r io.Reader) (map[string]ModelInfo, error) {
	var catalog map[string]struct {
		Models map[string]struct {
			Limit struct {
				Context int `json:"context"`
			} `json:"limit"`
			Cost *struct {
				Input      *float64 `json:"input"`
				Output     *float64 `json:"output"`
				CacheRead  *float64 `json:"cache_read"`
				CacheWrite *float64 `json:"cache_write"`
			} `json:"cost"`
		} `json:"models"`
	}
	if err := json.NewDecoder(r).Decode(&catalog); err != nil {
		return nil, err
	}
	out := map[string]ModelInfo{}
	for p, provider := range catalog {
		for id, m := range provider.Models {
			if m.Cost == nil || m.Cost.Input == nil || m.Cost.Output == nil {
				continue
			}
			out[p+"/"+id] = ModelInfo{Context: m.Limit.Context, Input: *m.Cost.Input, Output: *m.Cost.Output, CacheRead: m.Cost.CacheRead, CacheWrite: m.Cost.CacheWrite}
		}
	}
	return out, nil
}

func handleResponsesEvent(e SSEEvent, emit func(Event)) error {
	s := responsesStream{items: map[string]any{}}
	return s.handle(e, emit)
}
