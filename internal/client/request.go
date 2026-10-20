package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"meno/internal/proto"
	"net/http"
	"net/url"
	"strings"
)

// RemoteError represents a structured daemon response error.
type RemoteError struct {
	Status        int
	Code, Message string
}

func (e *RemoteError) Error() string { return e.Message }
func (c *Client) request(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return e
		}
		body = strings.NewReader(string(b))
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if e != nil {
		return e
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var env proto.ErrorEnvelope
		if e = json.NewDecoder(resp.Body).Decode(&env); e == nil {
			return &RemoteError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		return fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.request(ctx, http.MethodGet, path, nil, out)
}
func (c *Client) post(ctx context.Context, path string, in, out any) error {
	return c.request(ctx, http.MethodPost, path, in, out)
}
func (c *Client) patch(ctx context.Context, path string, in, out any) error {
	return c.request(ctx, http.MethodPatch, path, in, out)
}
func (c *Client) delete(ctx context.Context, path string, out any) error {
	return c.request(ctx, http.MethodDelete, path, nil, out)
}
func pathID(s string) string { return url.PathEscape(s) }
