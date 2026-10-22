package engine

import (
	"context"
	"encoding/json"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
	"sync"
	"testing"
)

// A snapshot and the replay after its cursor must partition the history: no turn
// may appear in both (duplicate) and none may be in neither (lost).
func TestSnapshotCursorMatchesState(t *testing.T) {
	for i := 0; i < 40; i++ {
		f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
		e, h := fixture(t, f)
		id := newSession(t, e)
		var wg sync.WaitGroup
		var snap api.Snapshot
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = e.Message(context.Background(), id, api.MessageRequest{Text: "hi", ClientID: "m"})
		}()
		go func() { defer wg.Done(); snap, _ = e.Snapshot(context.Background(), id) }()
		wg.Wait()
		waitFor(t, e, id, func(s *runState) bool { return !s.running })
		ch, stop, _ := h.Subscribe(id, snap.Seq, "boot")
		seen := map[string]bool{}
		for _, tr := range snap.Turns {
			seen[tr.ID] = true
		}
		for len(ch) > 0 {
			ev := <-ch
			if ev.Type != api.EventTurnAdded {
				continue
			}
			var d api.TurnAddedData
			_ = json.Unmarshal(ev.Data, &d)
			if seen[d.Turn.ID] {
				t.Fatalf("iteration %d: turn %s is in the snapshot and in the replay", i, d.Turn.ID)
			}
			seen[d.Turn.ID] = true
		}
		stop()
		if !seen["1"] {
			t.Fatalf("iteration %d: user turn lost", i)
		}
	}
}

// Runner-appended turns that are not yet published must not appear in an active
// session's list count; after publication list and snapshot agree.
func TestSnapshotAndListDoNotExposeUnpublishedTurns(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "first turn", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	<-f.seen
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "unpublished turn"})
	s.mu.Unlock()

	list, err := e.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Sessions) != 1 || list.Sessions[0].TurnCount != 1 {
		t.Fatalf("expected visible turn count 1, got %+v", list.Sessions)
	}
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TurnCount != 1 || snap.Session.TurnCount != 1 {
		t.Fatalf("expected snap turn count 1, got snap=%d session=%d", snap.TurnCount, snap.Session.TurnCount)
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	waitFor(t, e, id, func(st *runState) bool { return !st.running })

	afterList, err := e.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	afterSnap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if afterList.Sessions[0].TurnCount != afterSnap.TurnCount || afterSnap.Session.TurnCount != afterSnap.TurnCount {
		t.Fatalf("mismatch: list=%d snap=%d session=%d", afterList.Sessions[0].TurnCount, afterSnap.TurnCount, afterSnap.Session.TurnCount)
	}
}
