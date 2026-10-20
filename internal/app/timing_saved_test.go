package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func TestSavedTimingsAndSummaryAreDrawn(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	cost := 0.0123
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "go"})
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "step", CallID: "c1", Code: "work()", Output: "fine", ElapsedMs: 15000, LimitMs: 30000})
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "ready", Run: &agent.RunStats{Status: agent.RunOK, ElapsedMs: 21500, Turns: 2, ToolCalls: 1, ToolMs: 15000, Usage: llm.Usage{In: 2000, Out: 100, Cached: 1000}, Cost: &cost}})
	a.draw()
	text := screenText(s)
	for _, want := range []string{"✓ 15s", "✓ 22s · 2 turns · 1 tool call · 15s in tools", "↓2k ↑100 ↺50% $0.012"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	a.st.Settings.TurnSummary = "off"
	a.st.Settings.JSTiming = "off"
	a.draw()
	text = screenText(s)
	if strings.Contains(text, "2 turns") || strings.Contains(text, "50% of") {
		t.Fatalf("settings ignored:\n%s", text)
	}
	// The error background and the model's markdown are on the real screen.
	a.st.Settings.TurnSummary, a.st.Settings.JSTiming = "on", "all"
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "| a | b |\n|---|---|\n| **1** | 2 |", CallID: "c2", Code: "bad()", Err: "boom", ElapsedMs: 3})
	a.draw()
	_, h := s.Size()
	var sawErrBG, sawTable bool
	for y := 0; y < h; y++ {
		_, _, st, _ := s.GetContent(0, y)
		if _, bg, _ := st.Decompose(); bg == ui.BgJSErr {
			sawErrBG = true
		}
		if strings.Contains(visualRow(s, y), "│ 1 │ 2 │") {
			sawTable = true
		}
	}
	if !sawErrBG || !sawTable {
		t.Fatalf("error background %v, table %v:\n%s", sawErrBG, sawTable, screenText(s))
	}
}

// ---- whole turns through the runner ----

type usageClient struct {
	mu    sync.Mutex
	turns [][]llm.Event
	err   error
	block bool
}

func (c *usageClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	var evs []llm.Event
	if len(c.turns) > 0 {
		evs, c.turns = c.turns[0], c.turns[1:]
	}
	c.mu.Unlock()
	if c.block {
		<-ctx.Done()
		return ctx.Err()
	}
	for _, e := range evs {
		emit(e)
	}
	if len(evs) == 0 && c.err != nil {
		return c.err
	}
	return nil
}

func sendAndWait(t *testing.T, a *App, text string) {
	t.Helper()
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !a.st.Running })
}

func TestTurnEndsWithSummaryOnScreenAndInTheDatabase(t *testing.T) {
	cl := &usageClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "sh('sleep 0.1'); 1"}, {Kind: llm.EvDone, Usage: llm.Usage{In: 1000, Out: 50}}},
		{{Kind: llm.EvText, Text: "all **done**"}, {Kind: llm.EvDone, Usage: llm.Usage{In: 1200, Out: 20, Cached: 1000}}},
	}}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Pricing = map[string]llm.ModelInfo{"openai/gpt-5.5": {Context: 400000, Input: 1.25, Output: 10}}
	sendAndWait(t, a, "work")
	a.draw()
	text := screenText(scr)
	for _, want := range []string{"all done", "2 turns", "1 js call", "↓2.2k ↑70"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "✓ 0.") && !strings.Contains(text, "✓ 1") && !strings.Contains(text, "ms") {
		t.Fatalf("no duration:\n%s", text)
	}
	if !strings.Contains(text, "$0.00") {
		t.Fatalf("no cost although the model has a price:\n%s", text)
	}
	turns := a.st.Session.Snapshot()
	var call agent.Turn
	runs := 0
	for _, tr := range turns {
		if tr.CallID == "c1" {
			call = tr
		}
		if tr.Run != nil {
			runs++
		}
	}
	if call.ElapsedMs < 90 || call.LimitMs != 30000 {
		t.Fatalf("call timing %+v", call)
	}
	if runs != 1 || turns[len(turns)-1].Run == nil || turns[len(turns)-1].Run.Turns != 2 || turns[len(turns)-1].Run.Cost == nil {
		t.Fatalf("runs %d, last %+v", runs, turns[len(turns)-1].Run)
	}
	// What the screen showed is what the database holds.
	a.saveSession()
	loaded, err := agent.LoadSession(paths.Home, a.st.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	lt := loaded.Snapshot()
	if lt[len(lt)-1].Run == nil || lt[len(lt)-1].Run.ToolCalls != 1 {
		t.Fatalf("not saved: %+v", lt[len(lt)-1].Run)
	}
	for _, tr := range lt {
		if tr.CallID == "c1" && tr.ElapsedMs != call.ElapsedMs {
			t.Fatalf("call timing lost: %+v", tr)
		}
	}
}

func TestFailedTurnSaysWhyAndHowLong(t *testing.T) {
	cl := &usageClient{turns: [][]llm.Event{{{Kind: llm.EvError, Text: "http 401: invalid api key"}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.retry = agent.RetryPolicy{Attempts: 1}
	sendAndWait(t, a, "hello")
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "! http 401: invalid api key") || !strings.Contains(text, "✕ failed after") {
		t.Fatalf("%s", text)
	}
	last := a.st.Session.Snapshot()
	if r := last[len(last)-1].Run; r == nil || r.Status != agent.RunFail || r.Turns != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestStoppedTurnReportsItself(t *testing.T) {
	a, scr, _ := e2eApp(t, &usageClient{block: true})
	defer scr.Fini()
	typeRunes(a, "hello")
	a.onKey(keyK(tcell.KeyEnter))
	deadline := time.Now().Add(3 * time.Second)
	for a.st.Phase == "preparing" || a.st.Phase == "" {
		select {
		case msg := <-a.events:
			a.handle(msg)
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("the turn never started")
		}
	}
	a.abortTurn()
	pump(t, a, func() bool { return !a.st.Running })
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "■ stopped after") || !strings.Contains(text, "[aborted]") {
		t.Fatalf("%s", text)
	}
}

func TestTimedOutCallIsMarkedOnScreen(t *testing.T) {
	cl := &usageClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "for(;;){}"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "it hung"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Settings.JSTimeoutSec = 1
	a.st.VM.SetTimeout(300 * time.Millisecond)
	sendAndWait(t, a, "loop")
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "✕ 0.3s") || !strings.Contains(text, "it hung") {
		t.Fatalf("%s", text)
	}
}
