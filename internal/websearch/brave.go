package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// braveURL is a variable so tests can point it at a local server.
var braveURL = "https://api.search.brave.com/res/v1/web/search"

func braveSearch(ctx context.Context, key, query string, count int) (Answer, error) {
	if key == "" {
		return Answer{}, fmt.Errorf("brave needs a key (settings → Web → search key)")
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("count", strconv.Itoa(count))
	b, err := fetch(ctx, "GET", braveURL+"?"+q.Encode(), map[string]string{
		"Accept":               "application/json",
		"X-Subscription-Token": key,
	}, nil)
	if err != nil {
		return Answer{}, err
	}
	var out struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Answer{}, fmt.Errorf("brave: %w", err)
	}
	var res []Result
	for _, r := range out.Web.Results {
		res = append(res, Result{Title: cleanText(r.Title), URL: r.URL, Snippet: cleanText(r.Description)})
		if len(res) >= count {
			break
		}
	}
	if len(res) == 0 {
		return Answer{}, fmt.Errorf("brave: no results")
	}
	return Answer{Results: res}, nil
}
