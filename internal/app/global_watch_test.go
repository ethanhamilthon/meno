package app

import (
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/client"
	"meno/internal/proto"
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
	a.setAsk(background, &proto.AskForm{AskID: "ask", Questions: []proto.AskQuestion{{ID: "q", Prompt: "Question", Type: "text"}}})
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
	a.onGlobalWatch(globalWatchEvent{ev: proto.Event{Type: client.GlobalUnavailable}})
	if a.connLabel() == "" {
		t.Fatal("global disconnect invisible")
	}
	a.onGlobalSessions(globalSessionsLoaded{generation: a.globalWatch.generation})
	if a.connLabel() != "" {
		t.Fatal("hydration did not restore connection")
	}
}
