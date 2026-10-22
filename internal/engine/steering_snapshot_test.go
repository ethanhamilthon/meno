package engine

import (
	"context"
	"encoding/json"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
	"testing"
	"time"
)

type publicationBarrier struct {
	EventHub
	arrived chan struct{}
	release chan struct{}
}

func (b *publicationBarrier) BootID() string { return "boot" }
func (b *publicationBarrier) Publish(id, typ string, data any) api.Event {
	if typ == api.EventTurnAdded {
		close(b.arrived)
		<-b.release
	}
	return b.EventHub.Publish(id, typ, data)
}

func TestSteeringSnapshotWaitsForHistoryPublication(t *testing.T) {
	e, h := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.running = true
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "consumed"})
	s.mu.Unlock()
	barrier := &publicationBarrier{EventHub: h, arrived: make(chan struct{}), release: make(chan struct{})}
	e.events = barrier
	committed := make(chan struct{})
	go func() { e.agentEvent(s, agent.Ev{Kind: agent.EvInjected}); close(committed) }()
	<-barrier.arrived
	snapshots := make(chan api.Snapshot, 1)
	go func() { sn, _ := e.Snapshot(context.Background(), id); snapshots <- sn }()
	select {
	case <-snapshots:
		t.Fatal("snapshot crossed an incomplete publication")
	case <-time.After(20 * time.Millisecond):
	}
	close(barrier.release)
	<-committed
	snap := <-snapshots
	if len(snap.Turns) != 1 || len(snap.Queued) != 0 {
		t.Fatalf("inconsistent snapshot: %+v", snap)
	}
	replay, stop, _ := h.Subscribe(id, snap.Seq, "boot")
	defer stop()
	for len(replay) > 0 {
		if ev := <-replay; ev.Type == api.EventTurnAdded {
			t.Fatal("snapshot turn replayed after its cursor")
		}
	}
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func TestFinishTailPublishesNewRunAndConsumedTurn(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, h := fixture(t, f)
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.running = true
	s.requestID = "old"
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "first"})
	s.session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "finished"})
	s.queue.Push(agent.Message{Text: "tail"})
	s.mu.Unlock()
	e.finish(s, nil)
	select {
	case <-f.seen:
	case <-time.After(3 * time.Second):
		t.Fatal("tail did not launch")
	}
	snap, _ := e.Snapshot(context.Background(), id)
	if !snap.Running || snap.RequestID == "old" || len(snap.Queued) != 0 || len(snap.Turns) != 3 || snap.Turns[2].Content != "tail" {
		t.Fatalf("tail snapshot: %+v", snap)
	}
	replay, stop, _ := h.Subscribe(id, 0, "boot")
	defer stop()
	tailTurns := 0
	newStarted := false
	for len(replay) > 0 {
		ev := <-replay
		if ev.Type == api.EventTurnAdded {
			var d api.TurnAddedData
			_ = json.Unmarshal(ev.Data, &d)
			if d.Turn.Content == "tail" {
				tailTurns++
			}
		}
		if ev.Type == api.EventStatus {
			var d api.StatusData
			_ = json.Unmarshal(ev.Data, &d)
			if d.Running && d.RequestID == snap.RequestID {
				newStarted = true
			}
		}
		if newStarted && ev.Type == api.EventDone {
			t.Fatal("old terminal event followed new-run status")
		}
	}
	if tailTurns != 1 || !newStarted {
		t.Fatalf("tail turns=%d, new status=%v", tailTurns, newStarted)
	}
	_ = e.Stop(context.Background(), id, api.StopRequest{})
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
}
