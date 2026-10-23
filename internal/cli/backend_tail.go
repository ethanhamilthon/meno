package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"meno/internal/api"
)

const childPollInterval = time.Second

// Tail streams the events of the sessions in r to emit until each has ended or
// the process is interrupted. An idle session gives its turns instead.
func (a *DaemonAPI) Tail(r TailRequest, emit func(Event) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	boot := ""
	if h, err := a.c.Hello(ctx); err == nil {
		boot = h.BootID
	}
	var mu sync.Mutex
	want := map[string]bool{}
	for _, t := range r.Events {
		want[t] = true
	}
	out := func(e Event) error {
		if len(want) > 0 && !want[e.Type] {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		return emit(e)
	}
	if r.ChildrenOf != "" {
		return a.tailChildren(ctx, r, boot, out)
	}
	errs := make(chan error, len(r.IDs))
	for _, id := range r.IDs {
		go func() { errs <- a.tailOne(ctx, id, boot, r, out) }()
	}
	var first error
	for range r.IDs {
		if err := <-errs; err != nil && first == nil {
			first = err
		}
	}
	return first
}

// tailChildren follows the sessions that parent started: the running ones, and
// any that start while it watches. It ends when none of them is running.
func (a *DaemonAPI) tailChildren(ctx context.Context, r TailRequest, boot string, out func(Event) error) error {
	parent := r.ChildrenOf
	if parent == "me" {
		parent = a.session
	}
	var mu sync.Mutex
	attached := map[string]bool{}
	var wg sync.WaitGroup
	var first error
	quiet := 0
	for ctx.Err() == nil && quiet < 2 {
		list, err := a.Sessions(SessionsRequest{SpawnedBy: parent})
		if err != nil {
			return err
		}
		busy := false
		for _, s := range list {
			if s.Status == "idle" {
				continue
			}
			busy = true
			mu.Lock()
			if attached[s.ID] {
				mu.Unlock()
				continue
			}
			attached[s.ID] = true
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := a.tailOne(ctx, s.ID, boot, TailRequest{Events: r.Events}, out)
				mu.Lock()
				delete(attached, s.ID)
				if err != nil && first == nil {
					first = err
				}
				mu.Unlock()
			}()
		}
		mu.Lock()
		active := len(attached) > 0
		mu.Unlock()
		if busy || active {
			quiet = 0
		} else {
			quiet++
		}
		select {
		case <-ctx.Done():
		case <-time.After(childPollInterval):
		}
	}
	wg.Wait()
	return first
}

func tailEvent(id, boot string, e api.Event) Event {
	ev := toEvent(e)
	ev.Session, ev.Boot = id, boot
	if e.BootID != "" {
		ev.Boot = e.BootID
	}
	return ev
}

// tailOne streams one session. A running session gives its events from the
// current point, or after r.From of boot r.Boot; an idle one gives its turns
// (numbered from 1, skipping the first r.From).
func (a *DaemonAPI) tailOne(ctx context.Context, id, boot string, r TailRequest, out func(Event) error) error {
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return fmt.Errorf("session %s: %w", id, wrap(err))
	}
	if !snap.Running {
		for i, t := range snap.Turns {
			if r.HasFrom && uint64(i+1) <= r.From {
				continue
			}
			if err := out(Event{Session: id, Boot: boot, Sequence: uint64(i + 1), Type: "turn", Data: map[string]any{"role": t.Role, "text": turnText(t)}}); err != nil {
				return err
			}
		}
		return nil
	}
	after, since := snap.Seq, ""
	if r.HasFrom {
		after, since = r.From, r.Boot
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := false
	err = a.c.Stream(ctx, id, after, since, func(e api.Event) error {
		if err := out(tailEvent(id, boot, e)); err != nil {
			return err
		}
		if e.Type == api.EventDone || e.Type == api.EventError || e.Type == api.EventAborted {
			finished = true
			cancel()
			return errStop
		}
		return nil
	})
	if err != nil && !finished && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func turnText(t api.Turn) string {
	if t.Content != "" {
		return t.Content
	}
	for _, p := range t.Parts {
		if p.Text != "" {
			return p.Text
		}
		if p.Call != nil {
			return fmt.Sprintf("$ %s", p.Call.Command)
		}
	}
	return ""
}
