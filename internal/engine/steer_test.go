package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/llm"
)

func waitTurns(t *testing.T, e *Engine, id string, pred func([]api.Turn) bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := e.Snapshot(context.Background(), id)
		if err == nil && !snap.Running && pred(snap.Turns) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	snap, _ := e.Snapshot(context.Background(), id)
	t.Fatalf("condition not met; turns=%d running=%v", len(snap.Turns), snap.Running)
}

// A finished background task wakes an idle session with an event.
func TestTaskFinishedWakesTheSession(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{
		{{Kind: llm.EvText, Text: "saw the task end"}, {Kind: llm.EvDone}},
	}}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	data := json.RawMessage(`{"command":"echo bg-done","notify":true}`)
	if _, err := e.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/tasks/start", "", api.ResourceRequest{Data: data}); err != nil {
		t.Fatal(err)
	}
	waitTurns(t, e, id, func(ts []api.Turn) bool {
		for _, tr := range ts {
			if tr.Kind == "event" {
				return true
			}
		}
		return false
	})
}

// A session waiting for a task wait does not get a second notice for it.
func TestStopCancelsPendingEventsOfTheRequest(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	r, _ := e.Message(context.Background(), id, api.MessageRequest{Text: "go", ClientID: "m"})
	<-f.seen
	if err := e.Stop(context.Background(), id, api.StopRequest{RequestID: r.RequestID}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	ok := s.cancelled[r.RequestID]
	s.mu.Unlock()
	if !ok || strings.TrimSpace(r.RequestID) == "" {
		t.Fatal("the stopped request is not remembered")
	}
}

func TestHumanClientWithoutTokenHasFullAccess(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	if _, err := e.Resource(context.Background(), "GET", "/v1/sessions/"+id+"/tasks/list", "", api.ResourceRequest{}); err != nil {
		t.Fatalf("human access: %v", err)
	}
	if _, err := e.Resource(context.Background(), "GET", "/v1/sessions/"+id+"/tasks/list", "", api.ResourceRequest{Token: "wrong"}); err == nil {
		t.Fatal("a wrong token must be refused")
	}
}
