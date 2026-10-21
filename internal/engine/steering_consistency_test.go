package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/client"
	"meno/internal/llm"
	"meno/internal/proto"
	"sync"
	"testing"
	"time"
)

type steeringProvider struct {
	requests chan llm.Request
	next     chan struct{}
	mu       sync.Mutex
	calls    int
}

func (p *steeringProvider) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	p.mu.Lock()
	p.calls++
	n := p.calls
	p.mu.Unlock()
	if n == 1 {
		emit(llm.Event{Kind: llm.EvText, Text: "old answer"})
	}
	p.requests <- req
	select {
	case <-p.next:
	case <-ctx.Done():
		return ctx.Err()
	}
	if n == 2 {
		emit(llm.Event{Kind: llm.EvText, Text: "new answer"})
	}
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func receiveRequest(t *testing.T, p *steeringProvider) llm.Request {
	t.Helper()
	select {
	case req := <-p.requests:
		return req
	case <-time.After(3 * time.Second):
		t.Fatal("provider request timeout")
		return llm.Request{}
	}
}

func TestSteeringSnapshotBeforeNextDelta(t *testing.T) {
	e, h := fixture(t, &fakeProvider{})
	p := &steeringProvider{requests: make(chan llm.Request, 3), next: make(chan struct{}, 3)}
	e.client = func(llm.Provider) llm.Client { return p }
	id := newSession(t, e)
	ctx := context.Background()
	if _, err := e.Message(ctx, id, proto.MessageRequest{Text: "start"}); err != nil {
		t.Fatal(err)
	}
	receiveRequest(t, p)
	for i := 0; i < 2; i++ {
		if _, err := e.Message(ctx, id, proto.MessageRequest{Text: "unique steering"}); err != nil {
			t.Fatal(err)
		}
	}
	streaming, _ := e.Snapshot(ctx, id)
	if streaming.Live.Text != "old answer" || len(streaming.Queued) != 2 {
		t.Fatalf("enqueue discarded stream: %+v", streaming)
	}
	p.next <- struct{}{}
	req := receiveRequest(t, p)
	// Later requests include the user entry as history, not a second delivery.
	steeringEntries := 0
	for _, msg := range req.Msgs {
		if msg.Role == llm.RoleUser && msg.Text == "unique steering\n\nunique steering" {
			steeringEntries++
		}
	}
	if steeringEntries != 1 {
		t.Fatalf("request contains %d steering entries", steeringEntries)
	}
	snap, err := e.Snapshot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Queued) != 0 || snap.Live.Text != "" || snap.Live.Thinking != "" {
		t.Fatalf("stale live/queue: %+v", snap)
	}
	if len(snap.Turns) != 3 || snap.Turns[1].Content != "old answer" || snap.Turns[2].Content != "unique steering\n\nunique steering" {
		t.Fatalf("steering not consumed once: %+v", snap.Turns)
	}
	var observers [2]client.Mirror
	for i := range observers {
		observers[i].Load(snap)
	}
	p.next <- struct{}{}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	ch, stop, _ := h.Subscribe(id, snap.Seq, "boot")
	defer stop()
	for len(ch) > 0 {
		ev := <-ch
		for i := range observers {
			observers[i].Apply(ev)
		}
	}
	final, _ := e.Snapshot(ctx, id)
	for i := range observers {
		state := observers[i].Snapshot()
		if len(state.Turns) != len(final.Turns) || len(state.Queued) != 0 || state.Live.Text != "" || state.Running {
			t.Fatalf("observer %d diverged: %+v", i, state)
		}
	}
}

func TestSnapshotDoesNotExposeRunnerAppendBeforePublication(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.running = true
	s.session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "unpublished"})
	s.mu.Unlock()
	snap, _ := e.Snapshot(context.Background(), id)
	if len(snap.Turns) != 0 {
		t.Fatal("runner mutation leaked before its publication")
	}
	e.agentEvent(s, agent.Ev{Kind: agent.EvInjected})
	snap, _ = e.Snapshot(context.Background(), id)
	if len(snap.Turns) != 1 || snap.Turns[0].Content != "unpublished" {
		t.Fatal("committed history missing")
	}
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}
