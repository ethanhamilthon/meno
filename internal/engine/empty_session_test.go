package engine

import (
	"context"
	"encoding/json"
	"testing"

	"meno/internal/agent"
	"meno/internal/daemon"
	"meno/internal/llm"
	"meno/internal/proto"
)

func listed(t *testing.T, e *Engine) []proto.SessionItem {
	t.Helper()
	l, err := e.ListSessions(context.Background(), proto.SessionsListRequest{})
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
	if _, err := e.PatchSession(context.Background(), id, proto.PatchSessionRequest{Model: &model}); err != nil {
		t.Fatal(err)
	}
	if n := len(listed(t, e)); n != 0 {
		t.Fatalf("a patched empty session is listed (%d)", n)
	}
	if _, err := e.Snapshot(context.Background(), id); err != nil {
		t.Fatalf("an unsaved session must still be usable: %v", err)
	}
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hello there", ClientID: "m"}); err != nil {
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
		if (<-ch).Type == proto.EventSessionCreated {
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
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "name me now", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	<-f.seen // the run is in flight and not finished
	named := false
	for len(ch) > 0 {
		ev := <-ch
		if ev.Type != proto.EventSessionChanged {
			continue
		}
		var d proto.ChangedData
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
