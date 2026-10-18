package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
)

// barrier blocks every Stream until all expected providers are streaming at once.
type barrier struct {
	mu      sync.Mutex
	started map[string]bool
	want    int
	all     chan struct{}
}

type barrierClient struct {
	name string
	g    *barrier
}

func (c *barrierClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	g := c.g
	g.mu.Lock()
	g.started[c.name] = true
	if len(g.started) == g.want {
		close(g.all)
	}
	g.mu.Unlock()
	select {
	case <-g.all:
	case <-time.After(3 * time.Second):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
	emit(llm.Event{Kind: llm.EvText, Text: "ok " + c.name})
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func TestSessionsOnDifferentProvidersStreamAtTheSameTime(t *testing.T) {
	g := &barrier{started: map[string]bool{}, want: 2, all: make(chan struct{})}
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	st := config.NewStore(paths)
	if err := st.SaveProviders([]llm.Provider{
		{Name: "alpha", BaseURL: "http://a", APIKey: "k", Protocol: llm.ProtoChat},
		{Name: "beta", BaseURL: "http://b", APIKey: "k", Protocol: llm.ProtoResponses},
	}); err != nil {
		t.Fatal(err)
	}
	a.st.Providers, _ = st.LoadProviders()
	a.client = func(p llm.Provider) llm.Client { return &barrierClient{name: p.Name, g: g} }

	a.setSelection("alpha", "m-a", "low")
	typeRunes(a, "one")
	a.onKey(keyK(tcell.KeyEnter))
	first := a.st.Runtime
	a.runCommand("new")
	a.setSelection("beta", "m-b", "high")
	typeRunes(a, "two")
	a.onKey(keyK(tcell.KeyEnter))
	second := a.st.Runtime

	pump(t, a, func() bool { return !first.Running && !second.Running })
	if len(g.started) != 2 {
		t.Fatalf("providers did not run together: %v", g.started)
	}
	if first.Session.Provider != "alpha" || second.Session.Provider != "beta" {
		t.Fatalf("sessions mixed up: %q %q", first.Session.Provider, second.Session.Provider)
	}
}

func TestStoredSessionWithoutModelDoesNotFollowTheDefault(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	old := agent.NewSession(paths.Cwd, "")
	old.Provider, old.Model, old.Effort = "", "", ""
	if err := agent.SaveSession(paths.Home, old); err != nil {
		t.Fatal(err)
	}
	a.openSession(old.ID)
	if p, m, _ := a.selection(); p != "openai" || m != "gpt-5.5" {
		t.Fatalf("expected the default at open time, got %q %q", p, m)
	}
	a.runCommand("new")
	a.setSelection("openai", "gpt-6", "low")
	a.openSession(old.ID)
	if _, m, _ := a.selection(); m != "gpt-5.5" {
		t.Fatalf("the stored session followed the default to %q", m)
	}
}
