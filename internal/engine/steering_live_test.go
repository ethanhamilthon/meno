package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
	"testing"
	"time"
)

func TestSteeringEnqueueRecallAndCancelPreserveActiveCall(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	ctx := context.Background()
	if _, err := e.Message(ctx, id, api.MessageRequest{Text: "start"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.seen:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	s, _ := e.load(ctx, id)
	e.agentEvent(s, agent.Ev{Kind: agent.EvJSStart, CallID: "active", Code: "long command"})
	if _, err := e.Message(ctx, id, api.MessageRequest{Text: "pending"}); err != nil {
		t.Fatal(err)
	}
	snap, _ := e.Snapshot(ctx, id)
	if len(snap.Live.Calls) != 1 || len(snap.Queued) != 1 {
		t.Fatal("enqueue lost active call or queue")
	}
	e.queueResource(s, "DELETE")
	snap, _ = e.Snapshot(ctx, id)
	if len(snap.Live.Calls) != 1 || len(snap.Queued) != 0 {
		t.Fatal("recall lost active call or retained queue")
	}
	if _, err := e.Message(ctx, id, api.MessageRequest{Text: "cancelled pending"}); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(ctx, id, api.StopRequest{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	snap, _ = e.Snapshot(ctx, id)
	if len(snap.Queued) != 0 || len(snap.Live.Calls) != 0 || snap.Running {
		t.Fatalf("cancel retained pending state: %+v", snap)
	}
	for _, turn := range snap.Turns {
		if turn.Content == "cancelled pending" {
			t.Fatal("cancelled queue became a new user turn")
		}
	}
}

func TestSteeringThinkingOnlyCommitPreservesParallelCalls(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	e.agentEvent(s, agent.Ev{Kind: agent.EvThink, Thinking: "completed thought"})
	e.agentEvent(s, agent.Ev{Kind: agent.EvJSStart, CallID: "parallel", Code: "command"})
	s.session.Append(agent.Turn{Role: llm.RoleAssistant})
	e.agentEvent(s, agent.Ev{Kind: agent.EvInjected})
	snap, _ := e.Snapshot(context.Background(), id)
	if snap.Live.Thinking != "" || snap.Live.Text != "" || len(snap.Live.Calls) != 1 {
		t.Fatalf("thinking-only boundary is inconsistent: %+v", snap.Live)
	}
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}
