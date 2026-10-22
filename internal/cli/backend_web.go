package cli

import (
	"context"
	"net/http"
	"time"

	"meno/internal/api"
)

// WebSearch asks the daemon to search with the configured provider.
func (a *DaemonAPI) WebSearch(query string) (WebSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var out WebSearchResult
	err := a.resource(ctx, http.MethodPost, "web", "search", "", api.WebSearchRequest{Query: query}, &out)
	return out, err
}

// WebFetch asks the daemon to fetch a page; the network runs in the daemon.
func (a *DaemonAPI) WebFetch(url string) (WebPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var out WebPage
	err := a.resource(ctx, http.MethodPost, "web", "fetch", "", api.WebFetchRequest{URL: url}, &out)
	return out, err
}
