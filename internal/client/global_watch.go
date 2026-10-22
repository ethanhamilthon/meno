package client

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"time"
)

// GlobalUnavailable marks a transient global stream failure. The watcher retries;
// consumers retain their last state but must not present it as current.
const GlobalUnavailable = "global_unavailable"

// WatchGlobal follows global invalidations until cancellation or a daemon bye.
// Every hello is delivered after subscription is established, so consumers can
// hydrate without losing a change that races with their initial read.
func (c *Client) WatchGlobal(ctx context.Context, receive func(api.Event) error) error {
	var after, head uint64
	var boot string
	delay := 100 * time.Millisecond
	for {
		var callbackErr error
		err := c.Stream(ctx, "", after, boot, func(ev api.Event) error {
			if ev.Type == api.EventHello {
				var h struct {
					Boot string `json:"boot_id"`
					Seq  uint64 `json:"seq"`
				}
				if e := json.Unmarshal(ev.Data, &h); e != nil {
					return e
				}
				if boot != "" && boot != h.Boot {
					after = 0
				}
				boot, head = h.Boot, h.Seq
			} else if ev.Seq > 0 {
				if after > 0 && ev.Seq != after+1 {
					return errResync
				}
				after = ev.Seq
			}
			if e := receive(ev); e != nil {
				callbackErr = e
				return e
			}
			delay = 100 * time.Millisecond
			return nil
		})
		if callbackErr != nil {
			return callbackErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return receive(api.Event{Type: api.EventBye})
		}
		if err == errResync {
			// Stream consumes resync frames. The preceding hello supplies a safe
			// restart cursor; the next hello schedules fresh hydration.
			after = head
			continue
		}
		if e := receive(api.Event{Type: GlobalUnavailable}); e != nil {
			return e
		}
		if wait(ctx, delay) {
			return ctx.Err()
		}
		delay = nextDelay(delay)
	}
}
