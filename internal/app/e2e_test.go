package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
)

// scriptedClient replays one round of events per Stream call.
type scriptedClient struct {
	mu    sync.Mutex
	turns [][]llm.Event
}

// left is how many scripted rounds have not been used.
func (c *scriptedClient) left() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.turns)
}

func (c *scriptedClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	var evs []llm.Event
	if len(c.turns) > 0 {
		evs, c.turns = c.turns[0], c.turns[1:]
	}
	c.mu.Unlock()
	for _, e := range evs {
		emit(e)
	}
	return nil
}

func e2eApp(t testing.TB, cl llm.Client) (*App, tcell.SimulationScreen, config.Paths) {
	t.Helper()
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	scr.SetSize(96, 30)
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	st := config.NewStore(paths)
	if err := st.SaveProviders([]llm.Provider{{Name: "openai", BaseURL: "http://x", APIKey: "k", Protocol: llm.ProtoChat}}); err != nil {
		t.Fatal(err)
	}
	s := config.DefaultSettings()
	s.Provider = "openai"
	s.Model = "gpt-5.5"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	r := startRig(t, paths.Home, cl)
	a, err := New(scr, paths, r.cl)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	settle(t, a)
	return a, scr, paths
}

// newTestApp is an app on a daemon whose model answers nothing.
func newTestApp(t *testing.T) (*App, tcell.SimulationScreen) {
	t.Helper()
	a, scr, _ := e2eApp(t, &scriptedClient{})
	return a, scr
}

// pump processes queued events until the turn finishes or the deadline passes.
func pump(t *testing.T, a *App, until func() bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !until() {
		select {
		case msg := <-a.events:
			a.handle(msg)
		case <-deadline:
			t.Fatalf("timeout waiting for condition; running=%v", a.st.Running)
		}
	}
}

func TestE2ETurnWithJSToolAndPersistence(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: `console.log(1+1)`}},
		{{Kind: llm.EvText, Text: "done: 2"}, {Kind: llm.EvDone}},
	}}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()

	typeRunes(a, "calculate")
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return runDone(a) })

	a.draw()
	out := screenText(scr)
	if !strings.Contains(out, "done: 2") {
		t.Fatalf("final text missing:\n%s", out)
	}
	turns := a.st.Session.Snapshot()
	if len(turns) < 3 {
		t.Fatalf("expected user, js, assistant turns; got %d", len(turns))
	}
	hasCode := false
	for _, tr := range turns {
		if strings.Contains(tr.Code, "console.log(1+1)") {
			hasCode = true
		}
	}
	if !hasCode {
		t.Fatal("js tool call not recorded in session")
	}
	if n := len(cl.turns); n != 0 {
		t.Fatalf("%d scripted rounds left unused", n)
	}
	a.saveSession()
	list, err := agent.ListSessions(paths.Home)
	if err != nil || len(list) != 1 {
		t.Fatalf("sessions on disk: %d err=%v", len(list), err)
	}
}

func TestE2EAbortMarksSession(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "partial"}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	typeRunes(a, "long task")
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return runDone(a) })
	a.abortTurn()
	pump(t, a, func() bool { return runDone(a) })
	if a.st.Running {
		t.Fatal("still running after abort")
	}
}

func TestE2EProviderMissingShowsToast(t *testing.T) {
	cl := &scriptedClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.setSelection("", "no-such-model-xyz", "")
	typeRunes(a, "hello")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Running {
		t.Fatal("turn started without provider")
	}
	if !strings.Contains(a.lastNotice(), "provider not configured") {
		t.Fatalf("toast %q", a.lastNotice())
	}
}
