package llm

import "strings"

type Model struct {
	ID        string
	Provider  string
	Ctx       int
	MaxOut    int
	Reasoning bool
	PriceIn   float64
	PriceOut  float64
}

var catalog = []Model{
	{"gpt-5.5", "openai", 400000, 128000, true, 1.25, 10},
	{"gpt-5.5-mini", "openai", 400000, 128000, true, 0.25, 2},
	{"gpt-5.5-nano", "openai", 400000, 64000, true, 0.05, 0.4},
	{"gpt-4.1", "openai", 1000000, 32000, false, 2, 8},
	{"gpt-4.1-mini", "openai", 1000000, 32000, false, 0.4, 1.6},
	{"o4-mini", "openai", 200000, 100000, true, 1.1, 4.4},
	{"claude-sonnet-4-5", "anthropic", 200000, 64000, true, 3, 15},
	{"claude-opus-4-1", "anthropic", 200000, 32000, true, 15, 75},
	{"claude-haiku-4-5", "anthropic", 200000, 64000, true, 1, 5},
	{"gemini-2.5-pro", "google", 1000000, 65000, true, 1.25, 10},
	{"gemini-2.5-flash", "google", 1000000, 65000, true, 0.3, 2.5},
	{"deepseek-r1", "openrouter", 128000, 32000, true, 0.55, 2.19},
	{"deepseek-v3", "openrouter", 128000, 8000, false, 0.27, 1.1},
	{"qwen3-coder", "openrouter", 262000, 66000, false, 0.2, 0.8},
	{"llama3.1", "local", 128000, 4096, false, 0, 0},
	{"qwen2.5-coder", "local", 32000, 4096, false, 0, 0},
	{"mistral-small", "local", 32000, 4096, false, 0, 0},
	{"phi4", "local", 16000, 4096, false, 0, 0},
}

func Catalog() []Model { return catalog }

func Filter(query string) []Model {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return catalog
	}
	var out []Model
	for _, m := range catalog {
		if strings.Contains(strings.ToLower(m.ID), q) || strings.Contains(m.Provider, q) {
			out = append(out, m)
		}
	}
	return out
}

func Lookup(id string) (Model, bool) {
	for _, m := range catalog {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}
