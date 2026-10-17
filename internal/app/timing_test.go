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

func TestClock(t *testing.T) {
	for d, want := range map[time.Duration]string{-time.Second: "0:00", 0: "0:00", 999 * time.Millisecond: "0:00", 5 * time.Second: "0:05", 42 * time.Second: "0:42",
		61 * time.Second: "1:01", 12*time.Minute + 5*time.Second: "12:05", 59*time.Minute + 59*time.Second: "59:59", time.Hour: "1:00:00", 3723 * time.Second: "1:02:03", 100 * time.Hour: "100:00:00"} {
		if got := clock(d); got != want {
			t.Errorf("clock(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestTimingSettingsInTheDialog(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.draw()
	text := screenText(s)
	for _, want := range []string{"js timing", "slow only", "turn summary"} {
		if !strings.Contains(text, want) {
			t.Fatalf("%q missing:\n%s", want, text)
		}
	}
	a.st.Sel = settingIdx(t, "js_timing")
	if a.st.Settings.JSTiming != "all" {
		t.Fatalf("default %q", a.st.Settings.JSTiming)
	}
	a.onKey(keyK(tcell.KeyRight))
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.JSTiming != "off" {
		t.Fatalf("cycled to %q", a.st.Settings.JSTiming)
	}
	a.onKey(keyK(tcell.KeyLeft))
	if a.st.Settings.JSTiming != "slow" {
		t.Fatalf("back to %q", a.st.Settings.JSTiming)
	}
	// +/- change nothing on a setting without a number.
	a.onKey(key('+'))
	if a.st.Settings.JSTiming != "slow" {
		t.Fatal("+ changed js timing")
	}
	a.st.Sel = settingIdx(t, "turn_summary")
	a.onKey(key(' '))
	if a.st.Settings.TurnSummary != "off" {
		t.Fatalf("summary %q", a.st.Settings.TurnSummary)
	}
	// The choice is saved: a new app on the same files sees it.
	saved, err := a.store.LoadSettings()
	if err != nil || saved.JSTiming != "slow" || saved.TurnSummary != "off" {
		t.Fatalf("not persisted: %+v %v", saved, err)
	}
	if optLabel(&a.st.Settings, "js_timing", "slow") != "slow only" || optLabel(&a.st.Settings, "js_timing", "all") != "all" || optLabel(&a.st.Settings, "turn_summary", "on") != "on" {
		t.Fatal("option labels")
	}
}

func TestSettingsDialogFitsNarrowTerminals(t *testing.T) {
	for _, size := range [][2]int{{40, 14}, {30, 16}, {20, 20}} {
		a, s := newTestApp(t)
		s.SetSize(size[0], size[1])
		a.openOverlay(OverSettings)
		for i := range settingRows {
			a.st.Sel = i
			a.draw()
		}
		s.Fini()
	}
}

func TestJSOptsFollowSettings(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for set, want := range map[string]ui.TimingMode{"all": ui.TimingAll, "slow": ui.TimingSlow, "off": ui.TimingOff, "": ui.TimingAll, "nonsense": ui.TimingAll} {
		a.st.Settings.JSTiming = set
		if got := a.jsOpts().Timing; got != want {
			t.Errorf("js_timing %q → %v, want %v", set, got, want)
		}
	}
	for set, want := range map[string]bool{"on": true, "off": false, "": true} {
		a.st.Settings.TurnSummary = set
		if got := a.jsOpts().Summary; got != want {
			t.Errorf("turn_summary %q → %v, want %v", set, got, want)
		}
	}
}

func TestRunningText(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	if a.runningText() != "" {
		t.Fatal("text while idle")
	}
	a.st.Running = true
	if a.runningText() != "" {
		t.Fatal("text without a start time (a run that began before the clock existed)")
	}
	a.st.RunStart = time.Now().Add(-42 * time.Second)
	a.st.Phase = "thinking"
	if got := a.runningText(); got != "turn 1 · 0:42" {
		t.Fatalf("%q", got)
	}
	a.st.RunTurns, a.st.RunJS = 2, 1
	a.st.Usage.In, a.st.Usage.Out = 1500, 20
	a.st.RunUsage0 = llm.Usage{In: 300, Out: 10}
	if got := a.runningText(); got != "turn 3 · 0:42 · 1 js call · ↓ 1.2k ↑ 10" {
		t.Fatalf("%q", got)
	}
	a.st.RunJS = 4
	a.st.Phase = "running"
	if got := a.runningText(); !strings.HasPrefix(got, "turn 2 · 0:42 · 4 js calls") {
		t.Fatalf("the executing call belongs to the request just answered: %q", got)
	}
	a.st.RunTurns, a.st.Phase = 0, "running"
	if got := a.runningText(); !strings.HasPrefix(got, "turn 1 ") {
		t.Fatalf("%q", got)
	}
	// Tokens are counted for this run only, never negative.
	a.st.RunUsage0 = llm.Usage{In: 9000, Out: 9000}
	if got := a.runningText(); strings.Contains(got, "↓") {
		t.Fatalf("%q", got)
	}
	a.st.Running = false
	if a.runningText() != "" {
		t.Fatal("text after the run")
	}
}

func TestStatusBarAndPlaceholderShowTheClock(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Running, a.st.Phase = true, "running"
	a.st.RunStart = time.Now().Add(-65 * time.Second)
	a.st.RunTurns, a.st.RunJS = 2, 2
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "turn 2 · 1:05 · 2 js calls") || !strings.Contains(text, "running... 1:05") {
		t.Fatalf("%s", text)
	}
	if strings.Contains(visualRow(s, 35), "◫") {
		t.Fatal("the session totals replace the run in the status bar only when idle")
	}
	// A pending retry, a question and visual mode take the middle of the bar over.
	a.st.Retry = &retryState{Attempt: 1, Max: 3, Until: time.Now().Add(3 * time.Second), Reason: "http 503"}
	a.draw()
	if row := visualRow(s, 35); !strings.Contains(row, "retry 1/3") || strings.Contains(row, "turn 2") {
		t.Fatalf("%q", row)
	}
	a.st.Retry = nil
	a.st.Running, a.st.RunStart = false, time.Time{}
	a.draw()
	if row := visualRow(s, 35); !strings.Contains(row, "◫") || strings.Contains(row, "turn") {
		t.Fatalf("idle bar: %q", row)
	}
	if !strings.Contains(screenText(s), "give a task") {
		t.Fatal("idle placeholder")
	}
	// Without a start time the placeholder is as it was.
	a.st.Running = true
	a.st.Phase = "thinking"
	a.draw()
	if !strings.Contains(screenText(s), "thinking...") || strings.Contains(screenText(s), "thinking... 0") {
		t.Fatal("placeholder")
	}
}

func TestRunCountersFollowAgentEvents(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	a.st.RunStart = time.Now()
	a.onAgent(agent.Ev{Kind: agent.EvUsage, Usage: llm.Usage{In: 10, Out: 2}})
	a.onAgent(agent.Ev{Kind: agent.EvCode, CallID: "c1", Code: "x"})
	if len(a.st.LiveCalls) != 1 || !a.st.LiveCalls[0].Started.IsZero() || a.st.LiveCalls[0].Running {
		t.Fatalf("a call that is only streaming has not started: %+v", a.st.LiveCalls)
	}
	before := time.Now()
	a.onAgent(agent.Ev{Kind: agent.EvJSStart, CallID: "c1", Code: "x", Limit: 30 * time.Second})
	c := a.st.LiveCalls[0]
	if !c.Running || c.Limit != 30*time.Second || c.Started.Before(before) || time.Since(c.Started) > time.Second {
		t.Fatalf("%+v", c)
	}
	a.onAgent(agent.Ev{Kind: agent.EvJSEnd, CallID: "c1", Elapsed: time.Second})
	a.onAgent(agent.Ev{Kind: agent.EvUsage, Usage: llm.Usage{In: 20, Out: 3}})
	if a.st.RunTurns != 2 || a.st.RunJS != 1 || len(a.st.LiveCalls) != 0 {
		t.Fatalf("turns %d js %d live %d", a.st.RunTurns, a.st.RunJS, len(a.st.LiveCalls))
	}
	a.onTurnDone(nil)
	if !a.st.RunStart.IsZero() {
		t.Fatal("run clock still ticking")
	}
	if a.runningText() != "" {
		t.Fatal(a.runningText())
	}
}

func TestLiveCallShowsItsTimer(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "go"})
	a.st.Running, a.st.Phase = true, "running"
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "sh('sleep 9')", Running: true, Started: time.Now().Add(-12 * time.Second), Limit: 30 * time.Second}}
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "running 12s") || !strings.Contains(text, "of 30s limit") {
		t.Fatalf("%s", text)
	}
	// A call still being written has no timer; one with no limit has no bar.
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "sh('sleep 9')"}}
	a.draw()
	if strings.Contains(screenText(s), "running 0") || strings.Contains(screenText(s), "limit") {
		t.Fatalf("%s", screenText(s))
	}
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "x", Running: true, Started: time.Now().Add(-3 * time.Second)}}
	a.draw()
	if text := screenText(s); !strings.Contains(text, "running 3.0s") || strings.Contains(text, "█") || strings.Contains(text, "limit") {
		t.Fatalf("%s", text)
	}
	a.st.Settings.JSTiming = "off"
	a.draw()
	if strings.Contains(screenText(s), "running 3") {
		t.Fatal("timer shown with timing off")
	}
}

func TestSavedTimingsAndSummaryAreDrawn(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	cost := 0.0123
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "go"})
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "step", CallID: "c1", Code: "work()", Output: "fine", ElapsedMs: 15000, LimitMs: 30000})
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "ready", Run: &agent.RunStats{Status: agent.RunOK, ElapsedMs: 21500, Turns: 2, JSCalls: 1, JSMs: 15000, Usage: llm.Usage{In: 2000, Out: 100, Cached: 1000}, Cost: &cost}})
	a.draw()
	text := screenText(s)
	for _, want := range []string{"✓ 15s", "50% of 30s limit", "✓ 22s · 2 turns · 1 js call · 15s in js", "↓2k ↑100 ↺50% $0.012"} {
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
	if lt[len(lt)-1].Run == nil || lt[len(lt)-1].Run.JSCalls != 1 {
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
	if !strings.Contains(text, "timed out after 300ms") || !strings.Contains(text, "it hung") {
		t.Fatalf("%s", text)
	}
}
