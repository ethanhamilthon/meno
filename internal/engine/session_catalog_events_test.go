package engine

import (
	"context"
	"testing"

	"meno/internal/agent"
	"meno/internal/daemon"
	"meno/internal/llm"
	"meno/internal/proto"
)

func TestGlobalSessionChangeFiltering(t *testing.T) {
	// Meaningful changes invalidate catalog
	if !globalSessionChange(proto.EventTurnAdded, proto.TurnAddedData{}) {
		t.Fatal("turn added must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventSessionChanged, proto.ChangedData{Fields: []string{"history"}}) {
		t.Fatal("history replacement must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventSessionChanged, proto.ChangedData{Fields: []string{"name"}}) {
		t.Fatal("name change must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventStatus, proto.StatusData{}) {
		t.Fatal("status change must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventAskOpen, nil) {
		t.Fatal("ask open must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventAskClosed, nil) {
		t.Fatal("ask closed must invalidate global catalog")
	}
	if !globalSessionChange(proto.EventTasksChanged, proto.TasksChangedData{}) {
		t.Fatal("tasks changed must invalidate global catalog")
	}

	// Deltas and accounting events must NOT invalidate catalog
	for _, typ := range []string{proto.EventTextDelta, proto.EventThinkDelta, proto.EventCallDelta, proto.EventUsage} {
		if globalSessionChange(typ, nil) {
			t.Fatalf("%s must not invalidate global catalog", typ)
		}
	}
}

func TestUnsavedSessionHidesGlobalCatalogEvents(t *testing.T) {
	e, _ := fixture(t, done())
	g := daemon.NewGlobalHub("boot", 100)
	e.global = g
	id := newSession(t, e)

	ch, stop, _ := g.Subscribe(0, "boot")
	defer stop()

	// While unsaved, turn addition must not publish global status
	e.publish(id, proto.EventTurnAdded, proto.TurnAddedData{Turn: proto.Turn{ID: "1"}})
	if len(ch) != 0 {
		t.Fatalf("unsaved session published %d global events", len(ch))
	}

	// Persist the session
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "saved turn"})
	if err := e.save(s); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()

	// Draining creation event announced when session was saved
	select {
	case ev := <-ch:
		if ev.Type != proto.EventSessionCreated {
			t.Fatalf("expected EventSessionCreated, got %s", ev.Type)
		}
	default:
		t.Fatal("expected EventSessionCreated when session became persisted")
	}

	// Once persisted, turn added publishes global status
	e.publish(id, proto.EventTurnAdded, proto.TurnAddedData{Turn: proto.Turn{ID: "2"}})
	if len(ch) != 1 {
		t.Fatalf("persisted session expected 1 global event, got %d", len(ch))
	}
	ev := <-ch
	if ev.Type != proto.EventGlobalStatus {
		t.Fatalf("expected global status event, got %s", ev.Type)
	}
}
