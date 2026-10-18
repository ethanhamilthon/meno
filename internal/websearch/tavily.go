package websearch

import (
	"context"
	"encoding/json"
	"fmt"
)

// tavilyURL is a variable so tests can point it at a local server.
var tavilyURL = "https://api.tavily.com/search"

func tavilySearch(ctx context.Context, key, query string, count int) (Answer, error) {
	if key == "" {
		return Answer{}, fmt.Errorf("tavily needs a key (settings → Web → search key)")
	}
	body, err := json.Marshal(map[string]any{
		"api_key":        key,
		"query":          query,
		"max_results":    count,
		"include_answer": true,
	})
	if err != nil {
		return Answer{}, err
	}
	b, err := fetch(ctx, "POST", tavilyURL, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + key,
	}, body)
	if err != nil {
		return Answer{}, err
	}
	var out struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Answer{}, fmt.Errorf("tavily: %w", err)
	}
	ans := Answer{Answer: cleanText(out.Answer)}
	for _, r := range out.Results {
		ans.Results = append(ans.Results, Result{Title: cleanText(r.Title), URL: r.URL, Snippet: cleanText(r.Content)})
		if len(ans.Results) >= count {
			break
		}
	}
	if len(ans.Results) == 0 && ans.Answer == "" {
		return Answer{}, fmt.Errorf("tavily: no results")
	}
	return ans, nil
}
