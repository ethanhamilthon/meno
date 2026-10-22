package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/api"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Stream reads an event stream and reconnects from its last acknowledged sequence.
// Use an empty session ID for the global stream. The callback runs serially.
func (c *Client) Stream(ctx context.Context, sessionID string, after uint64, bootID string, receive func(api.Event) error) error {
	path := "/v1/events"
	if sessionID != "" {
		path = "/v1/sessions/" + pathID(sessionID) + "/events"
	}
	delay := 100 * time.Millisecond
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, eventURL(path, after, bootID), nil)
		if e != nil {
			return e
		}
		if after > 0 {
			req.Header.Set("Last-Event-ID", strconv.FormatUint(after, 10))
		}
		if bootID != "" {
			req.Header.Set("X-Meno-Boot-ID", bootID)
		}
		resp, e := c.HTTP.Do(req)
		if e != nil {
			if wait(ctx, delay) {
				return ctx.Err()
			}
			delay = nextDelay(delay)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("event stream returned HTTP %d", resp.StatusCode)
		}
		streamCtx, cancel := context.WithCancel(ctx)
		// The request itself was made with ctx, so cancelling streamCtx does not
		// interrupt a blocked body read: close the body when it ends.
		go func() { <-streamCtx.Done(); resp.Body.Close() }()
		idle := make(chan struct{}, 1)
		go closeWhenIdle(streamCtx, cancel, c.idleTimeout(), idle)
		var cbErr error
		e = c.readSSE(resp, func(ev api.Event) error {
			select {
			case idle <- struct{}{}:
			default:
			}
			if ev.Type != api.EventHello && ev.Seq > after {
				after = ev.Seq
			}
			if ev.Type == api.EventResync {
				return errResync
			}
			if ev.BootID != "" {
				bootID = ev.BootID
			}
			if ev.Type == api.EventHello {
				var h struct {
					BootID string `json:"boot_id"`
				}
				_ = json.Unmarshal(ev.Data, &h)
				if h.BootID != "" {
					bootID = h.BootID
				}
			}
			if ev.Type == api.EventBye {
				return errStreamClosed
			}
			if err := receive(ev); err != nil {
				cbErr = err
				return err
			}
			return nil
		})
		cancel()
		resp.Body.Close()
		if cbErr != nil {
			return cbErr
		}
		if e == errStreamClosed {
			return nil
		}
		if e == errResync {
			return errResync
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if wait(ctx, delay) {
			return ctx.Err()
		}
		delay = nextDelay(delay)
	}
}

var errStreamClosed = fmt.Errorf("stream ended")
var errResync = fmt.Errorf("stream requires resync")

func eventURL(path string, after uint64, boot string) string {
	u := "http://unix" + path + "?after=" + strconv.FormatUint(after, 10)
	if boot != "" {
		u += "&boot_id=" + url.QueryEscape(boot)
	}
	return u
}
func (c *Client) idleTimeout() time.Duration {
	if c.IdleTimeout > 0 {
		return c.IdleTimeout
	}
	return 45 * time.Second
}

// closeWhenIdle cancels the stream when nothing arrives for d. Every received
// event (the daemon also sends keepalives) restarts the wait through seen.
func closeWhenIdle(ctx context.Context, cancel context.CancelFunc, d time.Duration, seen <-chan struct{}) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-seen:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d)
		case <-timer.C:
			cancel()
			return
		}
	}
}
func nextDelay(d time.Duration) time.Duration {
	d *= 2
	if d > 2*time.Second {
		return 2 * time.Second
	}
	return d
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-t.C:
		return false
	}
}
func (c *Client) readSSE(resp *http.Response, receive func(api.Event) error) error {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 4096), 2<<20)
	var typ, id string
	var data strings.Builder
	dispatch := func() error {
		if typ == "" && data.Len() == 0 {
			return nil
		}
		ev := api.Event{Type: typ, Data: json.RawMessage(data.String())}
		if id != "" {
			ev.Seq, _ = strconv.ParseUint(id, 10, 64)
		}
		if len(ev.Data) == 0 {
			ev.Data = json.RawMessage([]byte("{}"))
		}
		typ = ""
		data.Reset()
		return receive(ev)
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if e := dispatch(); e != nil {
				return e
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimPrefix(val, " ")
		switch key {
		case "id":
			id = val
		case "event":
			typ = val
		case "data":
			if data.Len() > 0 {
				data.WriteByte(10)
			}
			data.WriteString(val)
		}
	}
	if e := sc.Err(); e != nil {
		return e
	}
	return dispatch()
}
