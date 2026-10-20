package app

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/daemon/cfgsvc"
	"meno/internal/db"
	"meno/internal/engine"
	"meno/internal/llm"
	"meno/internal/ui"
)

// rig is a real daemon in the test process: the engine with a scripted model,
// the config service and the HTTP server on a temp socket. The app under test
// is the same code that runs in the terminal, talking to it through a client.
type rig struct {
	home string
	eng  *engine.Engine
	cl   *client.Client
	srv  *daemon.Server
}

func startRig(t testing.TB, home string, model llm.Client) *rig {
	t.Helper()
	hub, global := daemon.NewHub("t", 5000), daemon.NewGlobalHub("t", 200)
	svc, err := cfgsvc.New(cfgsvc.Options{Paths: config.Paths{Home: home}})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Options{Home: home, EventHub: hub, GlobalHub: global, Resources: svc,
		ProviderClient: func(llm.Provider) llm.Client { return model }})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := daemon.New(daemon.Options{Home: home, Backend: eng, Hub: hub, Global: global, Version: "test", OnStart: svc.Start, OnStop: func() { eng.Close(); svc.Close() }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ctx)
	}()
	t.Cleanup(func() { cancel(); srv.Stop("test"); <-done; eng.Close(); db.CloseAll() })
	for i := 0; i < 400; i++ {
		if _, err := os.Stat(srv.Socket()); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cl := client.New(home)
	return &rig{home: home, eng: eng, cl: cl, srv: srv}
}

// recClient answers every request with "ok" and keeps the requests it saw.
type recClient struct {
	mu   sync.Mutex
	reqs []llm.Request
}

func (c *recClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	emit(llm.Event{Kind: llm.EvText, Text: "ok"})
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func (c *recClient) requests() []llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]llm.Request(nil), c.reqs...)
}

// say types a message, sends it and waits for the run to end.
func say(t *testing.T, a *App, text string) {
	t.Helper()
	before := assistantTurns(a)
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool {
		return !a.st.Running && a.st.sending == 0 && len(a.st.Queued) == 0 && assistantTurns(a) > before
	})
}

// assistantTurns counts the answers the session shows.
func assistantTurns(a *App) int {
	n := 0
	for _, t := range a.st.Session.turns {
		if t.Role == "assistant" {
			n++
		}
	}
	return n
}

// lastUser returns the text of the last user message of a request.
func lastUser(r llm.Request) string {
	for i := len(r.Msgs) - 1; i >= 0; i-- {
		if r.Msgs[i].Role == llm.RoleUser {
			return r.Msgs[i].Text
		}
	}
	return ""
}

// transcriptText is the timeline as plain text, and whether a call is running.
func transcriptText(a *App) (string, bool) {
	var b strings.Builder
	running := false
	for _, l := range a.transcriptWithSelection(80) {
		for _, s := range l.Segs {
			b.WriteString(s.Text)
		}
		running = running || l.BG == ui.BgJSRun
		b.WriteByte('\n')
	}
	return b.String(), running
}

func cursorVisible(s tcell.SimulationScreen) bool {
	_, _, vis := s.GetCursor()
	return vis
}

// runDone is true when the last message was answered: the daemon has stored the
// answer and the run is over.
func runDone(a *App) bool {
	rt := a.st.Runtime
	if rt.Running || len(rt.Queued) > 0 {
		return false
	}
	n := rt.Session.Len()
	return n >= 2 && rt.Session.turns[n-1].Role == "assistant"
}

// settle processes events until the first snapshot of the session arrived, so
// it cannot overwrite what a test then puts into the session by hand.
func settle(t testing.TB, a *App) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !a.st.Saved {
		select {
		case msg := <-a.events:
			a.handle(msg)
		case <-deadline:
			t.Fatal("the first snapshot never arrived")
		}
	}
}

// pumpFor handles queued events for d.
func pumpFor(a *App, d time.Duration) {
	end := time.After(d)
	for {
		select {
		case msg := <-a.events:
			a.handle(msg)
		case <-end:
			return
		}
	}
}
