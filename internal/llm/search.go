package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrSearchUnsupported means the provider protocol has no native web search.
var ErrSearchUnsupported = errors.New("web search is not supported by this provider")

// Source is a page a search answer cites.
type Source struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

// SearchAnswer is the provider's answer to a search with the pages it cites.
type SearchAnswer struct {
	Text    string
	Sources []Source
}

// Search asks the provider's own web search tool. Only the Responses protocol
// has one here; every other protocol returns ErrSearchUnsupported rather than
// guessing at a request shape. The request is a single non-streaming call.
func Search(ctx context.Context, p Provider, model, query string) (SearchAnswer, error) {
	if p.Protocol != ProtoResponses {
		return SearchAnswer{}, fmt.Errorf("%w (%s uses protocol %q)", ErrSearchUnsupported, p.Name, p.Protocol)
	}
	body := map[string]any{
		"model": model,
		"input": "Search the web and answer briefly, citing sources: " + query,
		"tools": []map[string]any{{"type": "web_search"}},
	}
	resp, err := post(ctx, p, "/responses", body, map[string]string{"Accept": "application/json"})
	if err != nil {
		return SearchAnswer{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return SearchAnswer{}, err
	}
	return parseSearch(raw)
}

func parseSearch(raw []byte) (SearchAnswer, error) {
	var doc struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Text        string `json:"text"`
				Annotations []struct {
					Type  string `json:"type"`
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return SearchAnswer{}, fmt.Errorf("unreadable search response: %w", err)
	}
	var out SearchAnswer
	seen := map[string]bool{}
	for _, item := range doc.Output {
		if item.Type != "message" {
			continue
		}
		for _, c := range item.Content {
			out.Text += c.Text
			for _, a := range c.Annotations {
				if a.Type == "url_citation" && a.URL != "" && !seen[a.URL] {
					seen[a.URL] = true
					out.Sources = append(out.Sources, Source{Title: a.Title, URL: a.URL})
				}
			}
		}
	}
	out.Text = strings.TrimSpace(out.Text)
	if out.Text == "" {
		return SearchAnswer{}, errors.New("the provider returned no search answer")
	}
	return out, nil
}
