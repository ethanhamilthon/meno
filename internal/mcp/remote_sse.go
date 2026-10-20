package mcp

import (
	"context"
	"fmt"
	"meno/internal/safe"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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
		defer safe.Recover(nil)
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
				target := base.ResolveReference(ref)
				if target.Scheme != base.Scheme || target.Host != base.Host {
					r.mu.Unlock()
					if !announced {
						announced = true
						ready <- fmt.Errorf("sse: cross-origin endpoint rejected")
					}
					return
				}
				r.target = target.String()
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
		c.markDone()
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
