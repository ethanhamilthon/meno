package client

import (
	"context"
	"meno/internal/api"
	"net/url"
	"strconv"
)

// Resource accesses one opaque daemon-owned route in session scope.
func (c *Client) Resource(ctx context.Context, method, path, query string, q api.ResourceRequest) (api.ResourceResponse, error) {
	if query != "" {
		path += "?" + query
	}
	var v api.ResourceResponse
	e := c.request(ctx, method, path, q, &v)
	return v, e
}

// SearchFiles asks the daemon to search workspace paths.
func (c *Client) SearchFiles(ctx context.Context, q api.FileSearchRequest) (api.FileSearchResponse, error) {
	p := "/v1/fs/search?root=" + url.QueryEscape(q.Root) + "&q=" + url.QueryEscape(q.Query) + "&limit=" + strconv.Itoa(q.Limit)
	var v api.FileSearchResponse
	e := c.get(ctx, p, &v)
	return v, e
}

// ConfigResource reads an opaque daemon-owned settings or configuration resource.
func (c *Client) ConfigResource(ctx context.Context, method, path string, data []byte) (api.ConfigResponse, error) {
	in := api.ResourceRequest{}
	if len(data) > 0 {
		in.Data = data
	}
	var v api.ConfigResponse
	e := c.request(ctx, method, path, in, &v)
	return v, e
}

// DebugRequests returns recent model requests without streaming their payloads.
func (c *Client) DebugRequests(ctx context.Context, sessionID string) (api.DebugRequestsResponse, error) {
	p := "/v1/debug/requests?session=" + url.QueryEscape(sessionID)
	var v api.DebugRequestsResponse
	e := c.get(ctx, p, &v)
	return v, e
}
