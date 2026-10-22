package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/api"
	"meno/internal/client"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGlobalHydrationCoalescesAndRejectsOldGeneration(t *testing.T) {
	entered, release := make(chan struct{}, 3), make(chan struct{}, 3)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		entered <- struct{}{}
		<-release
		fmt.Fprint(w, `{"data":{"sessions":[{"id":"new","cwd":"/","running":false,"needs_you":false}],"tasks":2}}`)
	}))
	defer server.Close()
	c := &client.Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}}}
	a := &App{cl: c, events: make(chan any, 10), rts: map[string]*Runtime{}}
	a.globalWatch.generation = 1
	a.invalidateGlobal()
	<-entered
	for i := 0; i < 200; i++ {
		a.invalidateGlobal()
	}
	if hits.Load() != 1 {
		t.Fatal("concurrent hydration")
	}
	a.globalWatch.generation = 2
	release <- struct{}{}
	m := (<-a.events).(globalSessionsLoaded)
	a.onGlobalSessions(m)
	if len(a.globalSessions) != 0 {
		t.Fatal("old hydration applied")
	}
	<-entered
	release <- struct{}{}
	a.onGlobalSessions((<-a.events).(globalSessionsLoaded))
	a.running.Wait()
	if hits.Load() != 2 || len(a.globalSessions) != 1 || a.globalTasks != 2 {
		t.Fatalf("reads=%d rows=%v tasks=%d", hits.Load(), a.globalSessions, a.globalTasks)
	}
	// A tick is not a hydration trigger; an explicit no-dirty call also does nothing.
	a.refreshGlobalState()
	if hits.Load() != 2 {
		t.Fatal("idle hydration")
	}
}

func TestProjectionGenerationAndPendingSend(t *testing.T) {
	a, rt := queueTestApp()
	rt.generation = 2
	rt.sending = 1
	u := client.ProjectionUpdate{Reset: true, Changed: client.ActivityChanged, Values: client.MirrorState{Running: false}}
	a.onProjection(projectionEv{sid: rt.ID(), generation: 1, update: u})
	if !rt.Running {
		t.Fatal("retired watcher changed runtime")
	}
	a.onProjection(projectionEv{sid: rt.ID(), generation: 2, update: u})
	if !rt.Running {
		t.Fatal("idle attach cancelled pending send")
	}
	rt.sending = 0
	a.onProjection(projectionEv{sid: rt.ID(), generation: 2, update: u})
	if rt.Running {
		t.Fatal("authoritative idle ignored")
	}
}

func TestBackgroundAskAndFailedSendPreserveActiveComposer(t *testing.T) {
	a, active := queueTestApp()
	active.Input = []rune("active draft")
	active.Cursor = 3
	active.Scroll = 8
	a.st.Sel = 7
	a.commandFailed = false
	background := &Runtime{Session: &sessView{id: "background"}, Input: []rune("background draft"), Cursor: 4}
	a.rts[background.ID()] = background
	before, _ := json.Marshal(struct {
		Input  string
		Cursor int
	}{string(active.Input), active.Cursor})
	a.setAsk(background, &api.AskForm{AskID: "ask", Questions: []api.AskQuestion{{ID: "q", Prompt: "Question", Type: "text"}}})
	if background.Ask == nil {
		t.Fatal("background ask absent")
	}
	a.closeAskFor(background, background.Ask)
	a.onSendResult(sendResult{sid: background.ID(), err: fmt.Errorf("offline"), msg: msgDraft{Text: "unsent"}})
	after, _ := json.Marshal(struct {
		Input  string
		Cursor int
	}{string(active.Input), active.Cursor})
	if string(before) != string(after) || active.Scroll != 8 || a.st.Sel != 7 || a.commandFailed || a.st.Runtime != active {
		t.Fatal("background event changed active composer")
	}
	if string(background.Input) != "background draft\n\nunsent" {
		t.Fatalf("background draft %q", string(background.Input))
	}
}

func TestGlobalUnavailableShowsStaleState(t *testing.T) {
	a, _ := queueTestApp()
	a.onGlobalWatch(globalWatchEvent{ev: api.Event{Type: client.GlobalUnavailable}})
	if a.connLabel() == "" {
		t.Fatal("global disconnect invisible")
	}
	a.onGlobalSessions(globalSessionsLoaded{generation: a.globalWatch.generation})
	if a.connLabel() != "" {
		t.Fatal("hydration did not restore connection")
	}
}
func TestGlobalHydrationCatalogLifecycleAndFailureRecovery(t *testing.T) {
	a, _ := queueTestApp()
	a.globalWatch.generation = 1

	// Initial hydration populates picker catalog
	initial := []api.SessionItem{
		{ID: "s1", Name: "first", Cwd: "/w1", Model: "m1", Provider: "p1", TurnCount: 2},
		{ID: "s2", Name: "second", Cwd: "/w2", Model: "m2", Provider: "p2", TurnCount: 5},
	}
	a.onGlobalSessions(globalSessionsLoaded{generation: 1, rows: initial})
	rows := a.sessionRows()
	if len(rows) != 2 || rows[0].Turns != 2 || rows[1].Turns != 5 {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	// Rename, turn count, model, cwd change replaces catalog
	updated := []api.SessionItem{
		{ID: "s1", Name: "renamed", Cwd: "/w1-new", Model: "m1-new", Provider: "p1", TurnCount: 4},
		{ID: "s2", Name: "second", Cwd: "/w2", Model: "m2", Provider: "p2", TurnCount: 5},
	}
	a.onGlobalSessions(globalSessionsLoaded{generation: 1, rows: updated})
	rows = a.sessionRows()
	if rows[0].Title != "renamed" || rows[0].Cwd != "/w1-new" || rows[0].Turns != 4 {
		t.Fatalf("updated catalog mismatch: %+v", rows[0])
	}

	// Create and delete change catalog
	added := append(updated, api.SessionItem{ID: "s3", Name: "third", TurnCount: 1})
	a.onGlobalSessions(globalSessionsLoaded{generation: 1, rows: added})
	if len(a.sessionRows()) != 3 {
		t.Fatalf("expected 3 rows after creation, got %d", len(a.sessionRows()))
	}
	deleted := []api.SessionItem{updated[0]}
	a.onGlobalSessions(globalSessionsLoaded{generation: 1, rows: deleted})
	if len(a.sessionRows()) != 1 || a.sessionRows()[0].ID != "s1" {
		t.Fatalf("expected 1 row after deletion, got %+v", a.sessionRows())
	}

	// Old generation does not roll back accepted state
	a.globalWatch.generation = 2
	a.onGlobalSessions(globalSessionsLoaded{generation: 1, rows: initial})
	if len(a.sessionRows()) != 1 || a.sessionRows()[0].ID != "s1" {
		t.Fatalf("old generation rolled back state: %+v", a.sessionRows())
	}

	// Failed hydration preserves rows and marks stale
	a.onGlobalSessions(globalSessionsLoaded{generation: 2, err: fmt.Errorf("network error")})
	if !a.globalWatch.stale || len(a.sessionRows()) != 1 {
		t.Fatalf("failed hydration lost rows or did not mark stale: stale=%v rows=%d", a.globalWatch.stale, len(a.sessionRows()))
	}

	// Successful retry restores fresh state
	fresh := []api.SessionItem{{ID: "s-fresh", Name: "fresh", TurnCount: 10}}
	a.onGlobalSessions(globalSessionsLoaded{generation: 2, rows: fresh})
	if a.globalWatch.stale || len(a.sessionRows()) != 1 || a.sessionRows()[0].ID != "s-fresh" {
		t.Fatalf("recovery failed: stale=%v rows=%+v", a.globalWatch.stale, a.sessionRows())
	}
}

func TestGlobalHydrationEventBurstCoalesces(t *testing.T) {
	entered, release := make(chan struct{}, 5), make(chan struct{}, 5)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		entered <- struct{}{}
		<-release
		fmt.Fprint(w, `{"data":{"sessions":[],"tasks":0}}`)
	}))
	defer server.Close()
	c := &client.Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}}}
	a := &App{cl: c, events: make(chan any, 10), rts: map[string]*Runtime{}}
	a.globalWatch.generation = 1

	// Invalidate once: starts in-flight request
	a.invalidateGlobal()
	<-entered

	// Fire event burst while request is blocked
	for i := 0; i < 50; i++ {
		a.onGlobalWatch(globalWatchEvent{ev: api.Event{Type: api.EventGlobalStatus}})
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 in-flight hit during burst, got %d", hits.Load())
	}

	// Complete initial request
	release <- struct{}{}
	a.onGlobalSessions((<-a.events).(globalSessionsLoaded))

	// Exactly one trailing request must follow
	<-entered
	release <- struct{}{}
	a.onGlobalSessions((<-a.events).(globalSessionsLoaded))
	a.running.Wait()

	if hits.Load() != 2 {
		t.Fatalf("expected exactly 1 initial + 1 trailing request (2 total), got %d", hits.Load())
	}
}
