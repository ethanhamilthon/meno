package client

import (
	"context"
	"fmt"
	"meno/internal/proto"
	"net/url"
	"strconv"
)

// Hello returns daemon lifecycle and protocol details.
func (c *Client) Hello(ctx context.Context) (proto.Hello, error) {
	var v proto.Hello
	e := c.get(ctx, "/v1/hello", &v)
	return v, e
}

// ListSessions lists sessions using optional filters.
func (c *Client) ListSessions(ctx context.Context, q proto.SessionsListRequest) (proto.SessionsListResponse, error) {
	p := "/v1/sessions?cwd=" + url.QueryEscape(q.Cwd) + "&q=" + url.QueryEscape(q.Query) + "&limit=" + strconv.Itoa(q.Limit)
	var v proto.SessionsListResponse
	e := c.get(ctx, p, &v)
	return v, e
}

// CreateSession creates a session.
func (c *Client) CreateSession(ctx context.Context, q proto.SessionCreateRequest) (proto.SessionCreateResponse, error) {
	var v proto.SessionCreateResponse
	e := c.post(ctx, "/v1/sessions", q, &v)
	return v, e
}

// Snapshot fetches one consistent session state.
func (c *Client) Snapshot(ctx context.Context, id string) (proto.Snapshot, error) {
	var v proto.Snapshot
	e := c.get(ctx, "/v1/sessions/"+pathID(id), &v)
	return v, e
}

// PatchSession updates session metadata.
func (c *Client) PatchSession(ctx context.Context, id string, q proto.PatchSessionRequest) (proto.SessionItem, error) {
	var v proto.SessionItem
	e := c.patch(ctx, "/v1/sessions/"+pathID(id), q, &v)
	return v, e
}

// DeleteSession deletes a session.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.delete(ctx, "/v1/sessions/"+pathID(id), nil)
}

// Fork creates a session fork.
func (c *Client) Fork(ctx context.Context, id string, q proto.ForkRequest) (proto.ForkResponse, error) {
	var v proto.ForkResponse
	e := c.post(ctx, "/v1/sessions/"+pathID(id)+"/fork", q, &v)
	return v, e
}

// Turns fetches one page of persisted turns.
func (c *Client) Turns(ctx context.Context, id string, q proto.TurnsPageRequest) (proto.TurnsPage, error) {
	p := fmt.Sprintf("/v1/sessions/%s/turns?from=%d&limit=%d", pathID(id), q.From, q.Limit)
	var v proto.TurnsPage
	e := c.get(ctx, p, &v)
	return v, e
}

// Message submits or queues a message.
func (c *Client) Message(ctx context.Context, id string, q proto.MessageRequest) (proto.MessageResponse, error) {
	var v proto.MessageResponse
	e := c.post(ctx, "/v1/sessions/"+pathID(id)+"/messages", q, &v)
	return v, e
}

// Stop stops active session work.
func (c *Client) Stop(ctx context.Context, id string, q proto.StopRequest) error {
	return c.post(ctx, "/v1/sessions/"+pathID(id)+"/stop", q, nil)
}

// Compact compacts session history.
func (c *Client) Compact(ctx context.Context, id string, q proto.CompactRequest) (proto.CompactResponse, error) {
	var v proto.CompactResponse
	e := c.post(ctx, "/v1/sessions/"+pathID(id)+"/compact", q, &v)
	return v, e
}

// AnswerAsk submits answers to an open ask.
func (c *Client) AnswerAsk(ctx context.Context, id, askID string, q proto.AskAnswerRequest) error {
	return c.post(ctx, "/v1/sessions/"+pathID(id)+"/asks/"+pathID(askID), q, nil)
}

// Shutdown asks the daemon to stop after it closes active event streams.
func (c *Client) Shutdown(ctx context.Context, reason string) error {
	return c.post(ctx, "/v1/shutdown", proto.ShutdownRequest{Reason: reason}, nil)
}
