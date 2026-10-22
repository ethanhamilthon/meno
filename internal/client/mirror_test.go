package client

import (
	"encoding/json"
	"meno/internal/api"
	"testing"
)

func TestMirrorSnapshotAndEvents(t *testing.T) {
	m := &Mirror{}
	m.Load(api.Snapshot{Session: api.SessionItem{ID: "s"}, Seq: 1, Turns: []api.Turn{{Role: "user", Content: "hi"}}})
	event := func(seq uint64, typ string, v any) {
		b, _ := json.Marshal(v)
		m.Apply(api.Event{Seq: seq, Type: typ, Data: b})
	}
	event(2, api.EventStatus, api.StatusData{Running: true, Phase: "thinking"})
	event(3, api.EventThinkDelta, api.TextDeltaData{Text: "hmm"})
	event(4, api.EventCallStart, api.CallStartData{CallID: "c", Command: "echo hi"})
	event(5, api.EventCallDelta, api.CallDeltaData{CallID: "c", Delta: "echo"})
	event(6, api.EventUsage, api.Metrics{InputTokens: 10})
	s := m.Snapshot()
	if !s.Running || s.Phase != "writing" || s.Live.Thinking != "hmm" || s.Live.Calls[0].Delta != "echo" || s.Metrics.InputTokens != 10 {
		t.Fatalf("state %#v", s)
	}
}
func TestMirrorTurnAndGapResync(t *testing.T) {
	m := &Mirror{}
	m.Load(api.Snapshot{Seq: 3, Live: api.LiveState{Text: "part"}})
	b, _ := json.Marshal(api.TurnAddedData{Turn: api.Turn{Role: "assistant", Content: "answer"}})
	m.Apply(api.Event{Seq: 4, Type: api.EventTurnAdded, Data: b})
	s := m.Snapshot()
	if len(s.Turns) != 1 || s.Live.Text != "" {
		t.Fatalf("turn not applied: %#v", s)
	}
	m.Apply(api.Event{Seq: 6, Type: api.EventDone})
	if !m.Snapshot().NeedsResync {
		t.Fatal("sequence gap did not request resync")
	}
}
