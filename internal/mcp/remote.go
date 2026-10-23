package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"meno/internal/safe"
	"net/http"
	"strings"
	"sync"
	"time"
)

const protocolVersion = "2025-06-18"

// readSSE parses a text/event-stream and calls fn for every complete event.
func readSSE(r io.Reader, fn func(event, data string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	var event string
	var data []string
	flush := func() {
		if len(data) > 0 {
			ev := event
			if ev == "" {
				ev = "message"
			}
			fn(ev, strings.Join(data, "\n"))
		}
		event, data = "", nil
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	flush()
	return sc.Err()
}

// remote carries JSON-RPC over HTTP. It implements io.Writer for conn.send: each
// Write is one message, posted in the background; a failed request completes its
// pending call with the error.
type remote struct {
	ctx     context.Context
	cancel  context.CancelFunc
	client  *http.Client
	headers map[string]string
	conn    *conn

	mu      sync.Mutex
	target  string // where messages are posted
	session string // Mcp-Session-Id of a streamable-http session
	version string
	legacy  bool // sse transport: answers come back on the event stream

	// reinit starts a new session after the server forgot the old one (HTTP 404
	// to a request that carried Mcp-Session-Id). Set by newRemote.
	reinit   func(ctx context.Context) error
	modern   bool                           // set before the first request
	specs    func(tool string) []headerSpec // x-mcp-header annotations of a tool
	reinitMu sync.Mutex
}

func (r *remote) newRequest(method, target string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(r.ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func (r *remote) Write(b []byte) (int, error) {
	body := bytes.TrimSpace(append([]byte(nil), b...))
	var head struct {
		ID     *int64 `json:"id"`
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &head)
	go func() {
		defer safe.Recover(nil)
		err := r.post(body)
		if err != nil && head.ID != nil && head.Method != "" {
			r.conn.fail(*head.ID, err)
		}
	}()
	return len(b), nil
}

func (r *remote) post(body []byte) error { return r.postContext(r.ctx, body) }

func (r *remote) postContext(ctx context.Context, body []byte) error {
	return r.postOnce(ctx, body, true)
}

// postOnce posts one message. When the server answers 404 to a request that
// carried a session ID, the session has expired: the transport specification
// requires a new initialize without a session ID. The 404 means the server did
// not dispatch the message, so it is sent again, once, on the new session.
func (r *remote) postOnce(ctx context.Context, body []byte, mayRecover bool) error {
	r.mu.Lock()
	target, session, version := r.target, r.session, r.version
	r.mu.Unlock()
	req, err := r.newRequest(http.MethodPost, target, body)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	var request struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	_ = json.Unmarshal(body, &request)
	if request.Method != "initialize" {
		req.Header.Set("MCP-Protocol-Version", version)
	}
	if r.modern {
		r.modernHeaders(req, body)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
		r.mu.Lock()
		r.session = id
		r.mu.Unlock()
	}
	if resp.StatusCode == http.StatusNotFound && session != "" && mayRecover && !r.legacy && r.reinit != nil && request.Method != "initialize" {
		resp.Body.Close()
		if err := r.recoverSession(ctx, session); err != nil {
			return fmt.Errorf("mcp: session expired and could not be restarted: %w", err)
		}
		return r.postOnce(ctx, body, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &httpError{Status: resp.StatusCode, Body: string(msg)}
	}
	if r.legacy {
		return nil
	}
	if resp.StatusCode == 202 || resp.StatusCode == 204 {
		if len(request.ID) > 0 && request.Method != "" {
			return fmt.Errorf("mcp: HTTP response has no RPC result")
		}
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "text/event-stream"):
		return readSSE(resp.Body, func(_, data string) { r.conn.handle([]byte(data)) })
	default:
		raw, err := io.ReadAll(io.LimitReader(resp.Body, (32<<20)+1))
		if err != nil {
			return err
		}
		if len(raw) > 32<<20 {
			return fmt.Errorf("mcp: HTTP result exceeds 32 MiB")
		}
		if !json.Valid(raw) {
			return fmt.Errorf("mcp: malformed HTTP JSON response")
		}
		raw = bytes.TrimSpace(raw)
		if bytes.HasPrefix(raw, []byte("[")) {
			var batch []json.RawMessage
			if err := json.Unmarshal(raw, &batch); err != nil {
				return err
			}
			for _, m := range batch {
				r.conn.handle(m)
			}
		} else {
			r.conn.handle(raw)
		}
	}
	return nil
}

func newRemote(ctx context.Context, s Server) (*remote, *Client) {
	ctx, cancel := context.WithCancel(ctx)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	r := &remote{ctx: ctx, cancel: cancel, client: &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, headers: s.Headers, target: s.URL, version: protocolVersion}
	r.conn = newConn(r)
	c := &Client{Server: s, conn: r.conn, done: make(chan struct{}), ctx: ctx, cancel: cancel}
	c.sendRemote = func(call context.Context, msg any) error {
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		request, cancel := context.WithCancel(call)
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		defer cancel()
		return r.postContext(request, body)
	}
	r.reinit = c.handshake
	c.setVersion = func(version string) { r.mu.Lock(); r.version = version; r.mu.Unlock() }
	c.conn.notify = c.notification
	c.closer = func() { r.cancel(); transport.CloseIdleConnections() }
	go func() { <-ctx.Done(); c.markDone() }()
	return r, c
}

// startHTTP connects to a streamable-http server. The session starts with the
// initialize request, so there is nothing to open beforehand.
func startHTTP(ctx context.Context, s Server) (*Client, error) {
	if s.wantsModern() {
		c, err := startHTTPModern(ctx, s)
		if err == nil || !fallsBackToLegacy(s, err) {
			return c, err
		}
	}
	_, c := newRemote(ctx, s)
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
