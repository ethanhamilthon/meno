package app

import (
	"context"
	"sync"
	"testing"

	"meno/internal/llm"
)

// steeringClient holds each request so the test can inspect queue consumption
// while the model still works, rather than after the run ends.
type steeringClient struct {
	mu                          sync.Mutex
	requests                    int
	first, second               chan struct{}
	releaseFirst, releaseSecond chan struct{}
}

func (c *steeringClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.requests++
	n := c.requests
	c.mu.Unlock()
	started, release := c.first, c.releaseFirst
	if n > 1 {
		started, release = c.second, c.releaseSecond
	}
	close(started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-release:
	}
	if n == 1 {
		emit(llm.Event{Kind: llm.EvTool, CallID: "step", Code: "console.log(1)"})
	} else {
		emit(llm.Event{Kind: llm.EvText, Text: "done"})
		emit(llm.Event{Kind: llm.EvDone})
	}
	return nil
}

func TestSteeringQueueClearsBeforeRunFinishes(t *testing.T) {
	cl := &steeringClient{first: make(chan struct{}), second: make(chan struct{}), releaseFirst: make(chan struct{}), releaseSecond: make(chan struct{})}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	release := func(ch chan struct{}) {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	t.Cleanup(func() { release(cl.releaseFirst); release(cl.releaseSecond) })
	a.send(msgDraft{Text: "start"})
	pump(t, a, func() bool {
		select {
		case <-cl.first:
			return true
		default:
			return false
		}
	})
	a.send(msgDraft{Text: "steer the next step"})
	pump(t, a, func() bool { return len(a.st.Queued) == 1 })
	if a.st.Queued[0] != "steer the next step" || a.queueHeight() == 0 {
		t.Fatalf("queued message is not visible: %q", a.st.Queued)
	}
	release(cl.releaseFirst)
	pump(t, a, func() bool {
		select {
		case <-cl.second:
			return len(a.st.Queued) == 0
		default:
			return false
		}
	})
	if !a.st.Running || a.queueHeight() != 0 {
		t.Fatal("the queue must clear while the next request is still running")
	}
	release(cl.releaseSecond)
	pump(t, a, func() bool { return runDone(a) })
}
