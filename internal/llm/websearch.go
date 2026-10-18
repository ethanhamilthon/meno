package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// WebSearchResult is one page found by a search.
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// WebSearchAnswer is the outcome of a provider-native web search.
type WebSearchAnswer struct {
	Answer  string
	Results []WebSearchResult
}

// NativeWebSearch asks the provider's own web search tool. Only the responses
// protocol has one; any error lets the caller use its own search backends.
func NativeWebSearch(ctx context.Context, p Provider, model, query string, maxResults int) (WebSearchAnswer, error) {
	if p.Protocol == ProtoChat {
		return WebSearchAnswer{}, errors.New("provider has no native web search")
	}
	if strings.TrimSpace(model) == "" {
		return WebSearchAnswer{}, errors.New("no model for native web search")
	}
	body := map[string]any{
		"model": model,
		"input": query,
		"tools": []any{map[string]any{"type": "web_search"}},
	}
	resp, err := post(ctx, p, "/responses", body, nil)
	if err != nil {
		return WebSearchAnswer{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return WebSearchAnswer{}, err
	}
	var out struct {
		Output []struct {
			Type    string          `json:"type"`
			Action  json.RawMessage `json:"action"`
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Annotations []struct {
					Type       string `json:"type"`
					URL        string `json:"url"`
					Title      string `json:"title"`
					StartIndex int    `json:"start_index"`
					EndIndex   int    `json:"end_index"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return WebSearchAnswer{}, fmt.Errorf("responses search: %w", err)
	}
	if out.Error != nil && out.Error.Message != "" {
		return WebSearchAnswer{}, errors.New(out.Error.Message)
	}
	var parts []string
	seen := map[string]bool{}
	var results []WebSearchResult
	add := func(r WebSearchResult) {
		if r.URL == "" || seen[r.URL] {
			return
		}
		seen[r.URL] = true
		if maxResults > 0 && len(results) >= maxResults {
			return
		}
		results = append(results, r)
	}
	for _, item := range out.Output {
		for _, c := range item.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				parts = append(parts, strings.TrimSpace(c.Text))
			}
			for _, an := range c.Annotations {
				if an.Type != "url_citation" || an.URL == "" {
					continue
				}
				snippet := ""
				if an.StartIndex >= 0 && an.EndIndex > an.StartIndex && an.EndIndex <= len(c.Text) {
					snippet = strings.TrimSpace(c.Text[an.StartIndex:an.EndIndex])
				}
				add(WebSearchResult{Title: an.Title, URL: an.URL, Snippet: snippet})
			}
		}
		if string(item.Action) == "" {
			continue
		}
		var action struct {
			Sources []struct {
				URL string `json:"url"`
			} `json:"sources"`
		}
		if json.Unmarshal(item.Action, &action) != nil {
			continue
		}
		for _, s := range action.Sources {
			add(WebSearchResult{Title: hostTitle(s.URL), URL: s.URL})
		}
	}
	return WebSearchAnswer{Answer: strings.Join(parts, "\n\n"), Results: results}, nil
}

// hostTitle is a readable title when a source only gives its URL.
func hostTitle(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return strings.TrimPrefix(u.Host, "www.")
}
