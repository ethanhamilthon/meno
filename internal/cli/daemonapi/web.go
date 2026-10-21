package daemonapi

import (
	"context"
	"net/http"
	"time"

	"meno/internal/cli"
	"meno/internal/proto"
)

// WebSearch asks the daemon to search with the configured provider.
func (a *API) WebSearch(query string) (cli.WebSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var out cli.WebSearchResult
	err := a.resource(ctx, http.MethodPost, "web", "search", "", proto.WebSearchRequest{Query: query}, &out)
	return out, err
}

// WebFetch asks the daemon to fetch a page; the network runs in the daemon.
func (a *API) WebFetch(url string) (cli.WebPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var out cli.WebPage
	err := a.resource(ctx, http.MethodPost, "web", "fetch", "", proto.WebFetchRequest{URL: url}, &out)
	return out, err
}
