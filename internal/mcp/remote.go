package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	legacy  bool   // sse transport: answers come back on the event stream
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
		err := r.post(body)
		if err != nil && head.ID != nil && head.Method != "" {
			r.conn.fail(*head.ID, err)
		}
	}()
	return len(b), nil
}

func (r *remote) post(body []byte) error {
	r.mu.Lock()
	target, session := r.target, r.session
	r.mu.Unlock()
	req, err := r.newRequest(http.MethodPost, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
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
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if r.legacy || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "text/event-stream"):
		return readSSE(resp.Body, func(_, data string) { r.conn.handle([]byte(data)) })
	default:
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		if err != nil {
			return err
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
	r := &remote{ctx: ctx, cancel: cancel, client: &http.Client{}, headers: s.Headers, target: s.URL}
	r.conn = newConn(r)
	c := &Client{Server: s, conn: r.conn, done: make(chan struct{})}
	c.closer = func() {
		r.cancel()
		select {
		case <-c.done:
		default:
			close(c.done)
		}
	}
	return r, c
}

// startHTTP connects to a streamable-http server. The session starts with the
// initialize request, so there is nothing to open beforehand.
func startHTTP(ctx context.Context, s Server) (*Client, error) {
	_, c := newRemote(ctx, s)
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// startSSE connects to a server using the older HTTP+SSE transport: a GET opens
// the event stream, whose first event names the endpoint to post requests to.
func startSSE(ctx context.Context, s Server) (*Client, error) {
	r, c := newRemote(ctx, s)
	r.legacy = true
	req, err := r.newRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := r.client.Do(req)
	if err != nil {
		c.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		c.Close()
		return nil, fmt.Errorf("sse: http %d", resp.StatusCode)
	}
	ready := make(chan error, 1)
	go func() {
		defer resp.Body.Close()
		announced := false
		err := readSSE(resp.Body, func(event, data string) {
			switch event {
			case "endpoint":
				base, perr := url.Parse(s.URL)
				ref, rerr := url.Parse(strings.TrimSpace(data))
				if perr != nil || rerr != nil {
					return
				}
				r.mu.Lock()
				r.target = base.ResolveReference(ref).String()
				r.mu.Unlock()
				if !announced {
					announced = true
					ready <- nil
				}
			case "message":
				r.conn.handle([]byte(data))
			}
		})
		if !announced {
			if err == nil {
				err = fmt.Errorf("sse: stream ended before the endpoint event")
			}
			ready <- err
		}
		select {
		case <-c.done:
		default:
			close(c.done)
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			c.Close()
			return nil, err
		}
	case <-time.After(10 * time.Second):
		c.Close()
		return nil, fmt.Errorf("sse: no endpoint event after 10s")
	case <-ctx.Done():
		c.Close()
		return nil, ctx.Err()
	}
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
