package client

import (
	"encoding/json"
	"meno/internal/proto"
	"testing"
)

func TestMirrorSnapshotAndEvents(t *testing.T) {
	m := &Mirror{}
	m.Load(proto.Snapshot{Session: proto.SessionItem{ID: "s"}, Seq: 1, Turns: []proto.Turn{{Role: "user", Content: "hi"}}})
	event := func(seq uint64, typ string, v any) {
		b, _ := json.Marshal(v)
		m.Apply(proto.Event{Seq: seq, Type: typ, Data: b})
	}
	event(2, proto.EventStatus, proto.StatusData{Running: true, Phase: "thinking"})
	event(3, proto.EventThinkDelta, proto.TextDeltaData{Text: "hmm"})
	event(4, proto.EventCallStart, proto.CallStartData{CallID: "c", Command: "echo hi"})
	event(5, proto.EventCallDelta, proto.CallDeltaData{CallID: "c", Delta: "echo"})
	event(6, proto.EventUsage, proto.Metrics{InputTokens: 10})
	s := m.Snapshot()
	if !s.Running || s.Phase != "writing" || s.Live.Thinking != "hmm" || s.Live.Calls[0].Delta != "echo" || s.Metrics.InputTokens != 10 {
		t.Fatalf("state %#v", s)
	}
}
func TestMirrorTurnAndGapResync(t *testing.T) {
	m := &Mirror{}
	m.Load(proto.Snapshot{Seq: 3, Live: proto.LiveState{Text: "part"}})
	b, _ := json.Marshal(proto.TurnAddedData{Turn: proto.Turn{Role: "assistant", Content: "answer"}})
	m.Apply(proto.Event{Seq: 4, Type: proto.EventTurnAdded, Data: b})
	s := m.Snapshot()
	if len(s.Turns) != 1 || s.Live.Text != "" {
		t.Fatalf("turn not applied: %#v", s)
	}
	m.Apply(proto.Event{Seq: 6, Type: proto.EventDone})
	if !m.Snapshot().NeedsResync {
		t.Fatal("sequence gap did not request resync")
	}
}
