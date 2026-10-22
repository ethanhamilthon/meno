package client

import (
	"meno/internal/api"
	"testing"
)

func TestMirrorLiveBoundariesPreserveParallelCalls(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1, Live: api.LiveState{Text: "partial", Thinking: "thought", Calls: []api.CallState{{CallID: "a"}, {CallID: "b"}}}})
	m.Apply(ev(2, api.EventInjected, api.InjectedData{Queued: []string{"pending"}}))
	m.Apply(ev(3, api.EventTurnAdded, api.TurnAddedData{Turn: api.Turn{ID: "user", Role: "user"}}))
	if s := m.Snapshot(); s.Live.Text != "partial" || len(s.Live.Calls) != 2 {
		t.Fatal("queue/user notification erased active response")
	}
	m.Apply(ev(4, api.EventTurnAdded, api.TurnAddedData{Turn: api.Turn{ID: "assistant", Role: "assistant"}}))
	if s := m.Snapshot(); s.Live.Text != "" || s.Live.Thinking != "" || len(s.Live.Calls) != 2 {
		t.Fatal("assistant boundary retained text or discarded parallel calls")
	}
	m.Apply(ev(5, api.EventCallEnd, api.CallEndData{CallID: "a"}))
	if s := m.Snapshot(); len(s.Live.Calls) != 1 || s.Live.Calls[0].CallID != "b" {
		t.Fatal("call completion did not preserve the other call")
	}
}

func TestMirrorEventAdvancesSnapshotVersionAndIgnoresDuplicates(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 4, Version: api.SessionVersion{BootID: "boot", Revision: 4}})
	event := ev(5, api.EventTextDelta, api.TextDeltaData{Text: "new"})
	event.BootID = "boot"
	m.Apply(event)
	m.Apply(event)
	m.Load(api.Snapshot{Seq: 4, Version: api.SessionVersion{BootID: "boot", Revision: 4}})
	if s := m.Snapshot(); s.Live.Text != "new" || s.NeedsResync || s.Seq != 5 {
		t.Fatalf("duplicate event or stale snapshot changed state: %+v", s)
	}
}
