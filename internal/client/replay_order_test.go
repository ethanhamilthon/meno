package client

import (
	"encoding/json"
	"meno/internal/api"
	"testing"
)

func ev(seq uint64, typ string, v any) api.Event {
	b, _ := json.Marshal(v)
	return api.Event{Seq: seq, Type: typ, Data: b}
}

// A turn already in the snapshot must not be appended again when replayed.
func TestMirrorIgnoresReplayedTurn(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 4, Turns: []api.Turn{{ID: "1", Role: "user"}}})
	m.Apply(ev(5, api.EventTurnAdded, api.TurnAddedData{Turn: api.Turn{ID: "1", Role: "user"}}))
	if n := len(m.Snapshot().Turns); n != 1 {
		t.Fatalf("turns=%d", n)
	}
}

// A delayed finish of the old run must not stop the run that started after it.
func TestMirrorIgnoresOldRunFinish(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1})
	m.Apply(ev(2, api.EventStatus, api.StatusData{Running: true, RequestID: "new"}))
	m.Apply(ev(3, api.EventDone, api.DoneData{RequestID: "old"}))
	if s := m.Snapshot(); !s.Running || s.RequestID != "new" {
		t.Fatalf("state %+v", s)
	}
	m.Apply(ev(4, api.EventDone, api.DoneData{RequestID: "new"}))
	if m.Snapshot().Running {
		t.Fatal("matching finish must end the run")
	}
}

// A refresh older than the applied state must not roll it back.
func TestMirrorRejectsOlderSnapshot(t *testing.T) {
	var m Mirror
	v := func(r uint64) api.SessionVersion { return api.SessionVersion{BootID: "b", Revision: r} }
	m.Load(api.Snapshot{Seq: 11, Version: v(11), Turns: []api.Turn{{ID: "1"}, {ID: "2"}}})
	m.Load(api.Snapshot{Seq: 10, Version: v(10), Turns: []api.Turn{{ID: "1"}}})
	if s := m.Snapshot(); s.Seq != 11 || len(s.Turns) != 2 {
		t.Fatalf("rolled back: seq=%d turns=%d", s.Seq, len(s.Turns))
	}
}

// session_changed carries the names of changed fields, not the session. It must
// ask for a fresh snapshot instead of blanking the title, directory and model.
func TestMirrorSessionChangedKeepsStateAndAsksForResync(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1, Cwd: "/work", Model: "m1", Session: api.SessionItem{ID: "s", Name: "Title", Cwd: "/work"}})
	m.Apply(ev(2, api.EventSessionChanged, api.ChangedData{Fields: []string{"name", "cwd"}}))
	s := m.Snapshot()
	if s.Cwd != "/work" || s.Model != "m1" || s.Session.Name != "Title" {
		t.Fatalf("state was wiped: %+v", s)
	}
	if !s.NeedsResync {
		t.Fatal("a changed field must trigger a resync")
	}
}

// Load copies what it keeps: a later event must not change the snapshot the
// caller still holds, or a reader of that snapshot races with the stream.
func TestLoadDoesNotShareLiveCallsWithTheSnapshot(t *testing.T) {
	snap := api.Snapshot{Live: api.LiveState{Calls: []api.CallState{{CallID: "c1"}}}, Seq: 1}
	var m Mirror
	m.Load(snap)
	data, _ := json.Marshal(api.CallDeltaData{CallID: "c1", Delta: "more"})
	m.Apply(api.Event{Seq: 2, Type: api.EventCallDelta, Data: data})
	if snap.Live.Calls[0].Delta != "" {
		t.Fatalf("the snapshot changed after Load: %q", snap.Live.Calls[0].Delta)
	}
	if got := m.Snapshot().Live.Calls[0].Delta; got != "more" {
		t.Fatalf("the mirror lost the delta: %q", got)
	}
}
