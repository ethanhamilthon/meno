// Package websearch calls the search backends meno can use without a model:
// Brave, Tavily and the keyless DuckDuckGo HTML endpoint.
package websearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Backend names one search API.
type Backend string

const (
	Brave      Backend = "brave"
	Tavily     Backend = "tavily"
	DuckDuckGo Backend = "duckduckgo"
)

// Result is one page of a search.
type Result struct {
	Title   string
	URL     string
	Snippet string
}

// Answer is what a backend returned: an optional generated answer and pages.
type Answer struct {
	Answer  string
	Results []Result
}

// client has a timeout of its own: ctx may have no deadline.
var client = &http.Client{Timeout: 20 * time.Second}

// Search runs query on one backend. A Brave or Tavily call needs key.
func Search(ctx context.Context, b Backend, key, query string, count int) (Answer, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Answer{}, fmt.Errorf("the search query is empty")
	}
	count = clampCount(count)
	switch b {
	case Brave:
		return braveSearch(ctx, key, query, count)
	case Tavily:
		return tavilySearch(ctx, key, query, count)
	case DuckDuckGo:
		return duckSearch(ctx, query, count)
	}
	return Answer{}, fmt.Errorf("unknown search backend %q", b)
}

func clampCount(n int) int {
	if n <= 0 {
		return 5
	}
	return min(n, 10)
}

// fetch reads a response body of at most 8 MiB. HTTP errors carry the start of
// the body: search APIs explain a bad key there.
func fetch(ctx context.Context, method, rawURL string, headers map[string]string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "meno (websearch)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return nil, fmt.Errorf("%s: HTTP %d: %s", backendHost(rawURL), resp.StatusCode, msg)
	}
	return b, nil
}

func backendHost(rawURL string) string {
	u := rawURL
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	return u
}
