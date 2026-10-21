package engine

import (
	"context"
	"encoding/json"
	"meno/internal/llm"
	"meno/internal/proto"
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
		var snap proto.Snapshot
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "m"})
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
			if ev.Type != proto.EventTurnAdded {
				continue
			}
			var d proto.TurnAddedData
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
