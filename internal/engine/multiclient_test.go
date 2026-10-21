package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"meno/internal/llm"
	"meno/internal/proto"
)

// observer follows one session the way a client does: a snapshot, then every
// event after its cursor. Whatever it ends with must equal the daemon's state.
type observer struct {
	mu    sync.Mutex
	turns map[string]int // turn id -> times seen
	last  uint64
}

func follow(t *testing.T, e *Engine, h interface {
	Subscribe(string, uint64, string) (<-chan proto.Event, func(), bool)
}, id string) *observer {
	t.Helper()
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	o := &observer{turns: map[string]int{}, last: snap.Seq}
	for _, tr := range snap.Turns {
		o.turns[tr.ID]++
	}
	ch, stop, resync := h.Subscribe(id, snap.Seq, snap.Version.BootID)
	if resync {
		t.Fatal("resync right after a snapshot")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range ch {
			o.mu.Lock()
			o.last = ev.Seq
			if ev.Type == proto.EventTurnAdded {
				var d proto.TurnAddedData
				_ = json.Unmarshal(ev.Data, &d)
				o.turns[d.Turn.ID]++
			}
			o.mu.Unlock()
		}
	}()
	t.Cleanup(func() { stop(); <-done })
	return o
}

// Several clients attach while turns are being added: each one must see every
// turn exactly once, whatever moment it attached at.
func TestManyClientsSeeEveryTurnExactlyOnce(t *testing.T) {
	steps := make([][]llm.Event, 4)
	for i := range steps {
		steps[i] = []llm.Event{{Kind: llm.EvText, Text: fmt.Sprint("answer ", i)}, {Kind: llm.EvDone}}
	}
	f := &fakeProvider{steps: steps}
	e, h := fixture(t, f)
	id := newSession(t, e)
	var mu sync.Mutex
	var obs []*observer
	attach := func() {
		o := follow(t, e, h, id)
		mu.Lock()
		obs = append(obs, o)
		mu.Unlock()
	}
	attach()
	for i := 0; i < 4; i++ {
		if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: fmt.Sprint("q", i), ClientID: fmt.Sprint("c", i)}); err != nil {
			t.Fatal(err)
		}
		attach() // a late client joins in the middle of the run
		waitFor(t, e, id, func(s *runState) bool { return !s.running })
	}
	time.Sleep(100 * time.Millisecond)
	final, _ := e.Snapshot(context.Background(), id)
	for i, o := range obs {
		o.mu.Lock()
		if len(o.turns) != len(final.Turns) {
			t.Errorf("client %d sees %d turns, daemon has %d", i, len(o.turns), len(final.Turns))
		}
		for tid, n := range o.turns {
			if n != 1 {
				t.Errorf("client %d saw turn %s %d times", i, tid, n)
			}
		}
		o.mu.Unlock()
	}
}
