package daemon

import (
	"context"
	"meno/internal/api"
	"testing"
)

func TestFakeBackendEmitsScriptedEvents(t *testing.T) {
	hub := NewHub("b", 20)
	b := NewFakeBackend(hub)
	s, e := b.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: "/tmp"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = b.Message(context.Background(), s.Session.ID, api.MessageRequest{Text: "hello", ClientID: "request"})
	if e != nil {
		t.Fatal(e)
	}
	ch, cancel, resync := hub.Subscribe(s.Session.ID, 0, "b")
	defer cancel()
	if resync {
		t.Fatal("unexpected resync")
	}
	types := []string{}
	for len(ch) > 0 {
		types = append(types, (<-ch).Type)
	}
	want := []string{api.EventStatus, api.EventTextDelta, api.EventTurnAdded, api.EventStatus, api.EventDone}
	if len(types) != len(want) {
		t.Fatalf("events %v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events %v", types)
		}
	}
}
