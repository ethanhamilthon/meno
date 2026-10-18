package app

import (
	"strings"
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
	a.onKey(keyK(tcell.KeyTab))
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

func TestStatusBarAndPlaceholderShowTheClock(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Running, a.st.Phase = true, "running"
	a.st.RunStart = time.Now().Add(-65 * time.Second)
	a.st.RunTurns, a.st.RunJS = 2, 2
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "running... 1:05") {
		t.Fatalf("%s", text)
	}
	if row := visualRow(s, 35); !strings.Contains(row, "◫") || strings.Contains(row, "turn 2") {
		t.Fatalf("the status bar shows the session, not the run: %q", row)
	}
	// A pending retry, a question and visual mode take the middle of the bar over.
	a.st.Retry = &retryState{Attempt: 1, Max: 3, Until: time.Now().Add(3 * time.Second), Reason: "http 503"}
	a.draw()
	if row := visualRow(s, 35); !strings.Contains(row, "retry 1/3") {
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
}

func TestLiveCallShowsItsTimer(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "go"})
	a.st.Running, a.st.Phase = true, "running"
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "sh('sleep 9')", Running: true, Started: time.Now().Add(-12 * time.Second), Limit: 30 * time.Second}}
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "12s/30s") {
		t.Fatalf("%s", text)
	}
	// A call still being written has no timer; one with no limit has no limit.
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "sh('sleep 9')"}}
	a.draw()
	if strings.Contains(screenText(s), "0.0s") {
		t.Fatalf("%s", screenText(s))
	}
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "x", Running: true, Started: time.Now().Add(-3 * time.Second)}}
	a.draw()
	if text := screenText(s); !strings.Contains(text, "3.0s") || strings.Contains(text, "3.0s/") {
		t.Fatalf("%s", text)
	}
	a.st.Settings.JSTiming = "off"
	a.draw()
	if strings.Contains(screenText(s), "3.0s") {
		t.Fatal("timer shown with timing off")
	}
}
