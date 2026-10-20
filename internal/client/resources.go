package client

import (
	"context"
	"meno/internal/proto"
	"net/url"
	"strconv"
)

// Resource accesses one opaque daemon-owned route in session scope.
func (c *Client) Resource(ctx context.Context, method, path, query string, q proto.ResourceRequest) (proto.ResourceResponse, error) {
	if query != "" {
		path += "?" + query
	}
	var v proto.ResourceResponse
	e := c.request(ctx, method, path, q, &v)
	return v, e
}

// SearchFiles asks the daemon to search workspace paths.
func (c *Client) SearchFiles(ctx context.Context, q proto.FileSearchRequest) (proto.FileSearchResponse, error) {
	p := "/v1/fs/search?root=" + url.QueryEscape(q.Root) + "&q=" + url.QueryEscape(q.Query) + "&limit=" + strconv.Itoa(q.Limit)
	var v proto.FileSearchResponse
	e := c.get(ctx, p, &v)
	return v, e
}

// Config reads an opaque daemon-owned settings or configuration resource.
func (c *Client) Config(ctx context.Context, method, path string, data []byte) (proto.ConfigResponse, error) {
	in := proto.ResourceRequest{}
	if len(data) > 0 {
		in.Data = data
	}
	var v proto.ConfigResponse
	e := c.request(ctx, method, path, in, &v)
	return v, e
}

// DebugRequests returns recent model requests without streaming their payloads.
func (c *Client) DebugRequests(ctx context.Context, sessionID string) (proto.DebugRequestsResponse, error) {
	p := "/v1/debug/requests?session=" + url.QueryEscape(sessionID)
	var v proto.DebugRequestsResponse
	e := c.get(ctx, p, &v)
	return v, e
}
