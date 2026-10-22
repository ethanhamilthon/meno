package engine

import (
	"context"
	"encoding/json"
	"testing"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/daemon"
	"meno/internal/llm"
)

func listed(t *testing.T, e *Engine) []api.SessionItem {
	t.Helper()
	l, err := e.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return l.Sessions
}

func TestNewSessionIsNotStoredUntilFirstMessage(t *testing.T) {
	e, _ := fixture(t, done())
	g := daemon.NewGlobalHub("boot", 100)
	e.global = g
	id := newSession(t, e)
	if n := len(listed(t, e)); n != 0 {
		t.Fatalf("an empty session is listed (%d)", n)
	}
	model := "other"
	if _, err := e.PatchSession(context.Background(), id, api.PatchSessionRequest{Model: &model}); err != nil {
		t.Fatal(err)
	}
	if n := len(listed(t, e)); n != 0 {
		t.Fatalf("a patched empty session is listed (%d)", n)
	}
	if _, err := e.Snapshot(context.Background(), id); err != nil {
		t.Fatalf("an unsaved session must still be usable: %v", err)
	}
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "hello there", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	rows := listed(t, e)
	if len(rows) != 1 || rows[0].ID != id || rows[0].Name != "hello there" {
		t.Fatalf("rows %+v", rows)
	}
	ch, stop, _ := g.Subscribe(0, "boot")
	defer stop()
	created := 0
	for len(ch) > 0 {
		if (<-ch).Type == api.EventSessionCreated {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("session.created announced %d times", created)
	}
}

func TestDeleteUnsavedSessionIsQuiet(t *testing.T) {
	e, _ := fixture(t, done())
	id := newSession(t, e)
	if err := e.DeleteSession(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Snapshot(context.Background(), id); err == nil {
		t.Fatal("deleted session still answers")
	}
}

func TestTaskInEmptySessionStoresIt(t *testing.T) {
	e, _ := fixture(t, done())
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	err := e.ensureSaved(s)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed(t, e)) != 0 {
		// A stored session without a message is still hidden from the list.
		t.Fatal("stored empty session is listed")
	}
}

func TestFirstMessageNamesSessionBeforeTheRunEnds(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, hub := fixture(t, f)
	id := newSession(t, e)
	ch, stop, _ := hub.Subscribe(id, 0, "boot")
	defer stop()
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "name me now", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	<-f.seen // the run is in flight and not finished
	named := false
	for len(ch) > 0 {
		ev := <-ch
		if ev.Type != api.EventSessionChanged {
			continue
		}
		var d api.ChangedData
		_ = json.Unmarshal(ev.Data, &d)
		for _, f := range d.Fields {
			named = named || f == "name"
		}
	}
	if !named {
		t.Fatal("no name change was published before the run ended")
	}
	sn, err := e.Snapshot(context.Background(), id)
	if err != nil || sn.Session.Name != "name me now" {
		t.Fatalf("name %q err %v", sn.Session.Name, err)
	}
}

// usedSession is a session that already holds a message, so it is stored.
func usedSession(t *testing.T, e *Engine) string {
	t.Helper()
	id := newSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "earlier"})
	err = e.save(s)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// A run names the window that started it, in the status event and in the
// snapshot a late window reads; a run without an origin stays anonymous.
func TestRunKeepsTheOriginOfItsWindow(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, hub := fixture(t, f)
	id := newSession(t, e)
	ch, stop, _ := hub.Subscribe(id, 0, "boot")
	defer stop()
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "go", ClientID: "r1", Origin: "window-a"}); err != nil {
		t.Fatal(err)
	}
	<-f.seen
	var got string
	for len(ch) > 0 {
		ev := <-ch
		if ev.Type == api.EventStatus {
			var d api.StatusData
			_ = json.Unmarshal(ev.Data, &d)
			if d.Running && d.Origin != "" {
				got = d.Origin
			}
		}
	}
	if got != "window-a" {
		t.Fatalf("status origin %q", got)
	}
	sn, err := e.Snapshot(context.Background(), id)
	if err != nil || sn.Origin != "window-a" {
		t.Fatalf("snapshot origin %q err %v", sn.Origin, err)
	}
}

// A snapshot is encoded after the session lock is released, so it must not
// share the live call list the running turn keeps changing.
func TestSnapshotDoesNotShareLiveCallsWithTheRun(t *testing.T) {
	e, _ := fixture(t, done())
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.live = api.LiveState{Calls: []api.CallState{{CallID: "c1", Command: "ls"}}}
	s.mu.Unlock()
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.live.Calls[0].Command = "changed by the run"
	s.live.Calls = append(s.live.Calls, api.CallState{CallID: "c2"})
	s.mu.Unlock()
	if snap.Live.Calls[0].Command != "ls" || len(snap.Live.Calls) != 1 {
		t.Fatalf("the snapshot follows the run: %+v", snap.Live.Calls)
	}
}
