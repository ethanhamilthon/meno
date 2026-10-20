package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/proto"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Stream reads an event stream and reconnects from its last acknowledged sequence.
// Use an empty session ID for the global stream. The callback runs serially.
func (c *Client) Stream(ctx context.Context, sessionID string, after uint64, bootID string, receive func(proto.Event) error) error {
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
		if resp.Request != nil {
			resp.Request = resp.Request.WithContext(streamCtx)
		}
		go closeWhenIdle(streamCtx, cancel)
		e = c.readSSE(resp, func(ev proto.Event) error {
			if ev.Type != proto.EventHello && ev.Seq > after {
				after = ev.Seq
			}
			if ev.Type == proto.EventResync {
				return errResync
			}
			if ev.BootID != "" {
				bootID = ev.BootID
			}
			if ev.Type == proto.EventHello {
				var h struct {
					BootID string `json:"boot_id"`
				}
				_ = json.Unmarshal(ev.Data, &h)
				if h.BootID != "" {
					bootID = h.BootID
				}
			}
			if ev.Type == proto.EventBye {
				return errStreamClosed
			}
			return receive(ev)
		})
		cancel()
		resp.Body.Close()
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
func closeWhenIdle(ctx context.Context, cancel context.CancelFunc) {
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
		cancel()
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
func (c *Client) readSSE(resp *http.Response, receive func(proto.Event) error) error {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 4096), 2<<20)
	var typ, id string
	var data strings.Builder
	dispatch := func() error {
		if typ == "" && data.Len() == 0 {
			return nil
		}
		ev := proto.Event{Type: typ, Data: json.RawMessage(data.String())}
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
