package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Model is a model id reported by a provider's /models endpoint.
type Model struct {
	ID       string
	Provider string
}

var modelsHTTP = &http.Client{Timeout: 15 * time.Second}

// FetchModels calls GET {base_url}/models and returns the sorted model ids.
// Works for OpenAI-compatible endpoints (OpenAI, OpenRouter, Ollama, LM Studio).
func FetchModels(ctx context.Context, p Provider) ([]Model, error) {
	if err := Validate(p); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := modelsHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return parseModels(p.Name, body)
}

func parseModels(provider string, body []byte) ([]Model, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("models: bad response: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, errors.New("models: provider returned no models")
	}
	ms := make([]Model, 0, len(out.Data))
	for _, d := range out.Data {
		if d.ID != "" {
			ms = append(ms, Model{ID: d.ID, Provider: provider})
		}
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
	return ms, nil
}

// FilterModels returns models whose id contains query (case-insensitive).
func FilterModels(ms []Model, query string) []Model {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return ms
	}
	var out []Model
	for _, m := range ms {
		if strings.Contains(strings.ToLower(m.ID), q) {
			out = append(out, m)
		}
	}
	return out
}
