package tui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
)

func TestClock(t *testing.T) {
	for d, want := range map[time.Duration]string{-time.Second: "0:00", 0: "0:00", 999 * time.Millisecond: "0:00", 5 * time.Second: "0:05", 42 * time.Second: "0:42",
		61 * time.Second: "1:01", 12*time.Minute + 5*time.Second: "12:05", 59*time.Minute + 59*time.Second: "59:59", time.Hour: "1:00:00", 3723 * time.Second: "1:02:03", 100 * time.Hour: "100:00:00"} {
		if got := clock(d); got != want {
			t.Errorf("clock(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestAlwaysVisibleRunMetadataHasNoMisleadingToggle(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.onKey(keyK(tcell.KeyTab))
	a.draw()
	text := screenText(s)
	for _, old := range []string{"bash timing", "turn summary"} {
		if strings.Contains(text, old) {
			t.Fatal("obsolete visibility toggle", old)
		}
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
	for set, want := range map[string]bool{"on": true, "off": false, "": true} {
		a.st.Settings.TurnSummary = set
		if got := a.jsOpts().Summary; got != want {
			t.Errorf("turn_summary %q → %v, want %v", set, got, want)
		}
	}
}

func TestStatusBarCountsAndBusyPlaceholder(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Running = true
	a.st.Phase = "running"
	a.st.RunStart = time.Now().Add(-65 * time.Second)
	a.draw()
	if got := screenText(s); !strings.Contains(got, "Tell the working agent something") || strings.Contains(got, "1:05") {
		t.Fatal(got)
	}
	a.globalTasks = 1
	a.globalSessions = []api.SessionItem{{Running: true}, {Running: true, NeedsYou: true}, {Completed: 2}}
	a.draw()
	row := visualRow(s, 35)
	for _, want := range []string{"running", "tasks", "Asking", "done"} {
		if !strings.Contains(row, want) {
			t.Fatal(row)
		}
	}
	if strings.Contains(row, "◫") {
		t.Fatal("context remains in bar")
	}
}

func wireEvent(typ string, v any) api.Event {
	b, _ := json.Marshal(v)
	return api.Event{Type: typ, Data: b}
}

func TestRunCountersFollowAgentEvents(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	a.st.RunStart = time.Now()
	rt := a.st.Runtime
	a.applyWire(rt, wireEvent(api.EventUsage, api.UsageData{InputTokens: 10, OutputTokens: 2}))
	a.applyWire(rt, wireEvent(api.EventCallDelta, api.CallDeltaData{CallID: "c1", Delta: "x"}))
	if len(a.st.LiveCalls) != 1 || !a.st.LiveCalls[0].Started.IsZero() || a.st.LiveCalls[0].Running {
		t.Fatalf("a call that is only streaming has not started: %+v", a.st.LiveCalls)
	}
	before := time.Now()
	a.applyWire(rt, wireEvent(api.EventCallStart, api.CallStartData{CallID: "c1", Command: "x"}))
	c := a.st.LiveCalls[0]
	if !c.Running || c.Started.Before(before) || time.Since(c.Started) > time.Second {
		t.Fatalf("%+v", c)
	}
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c1", ElapsedMS: 1000}))
	a.applyWire(rt, wireEvent(api.EventUsage, api.UsageData{InputTokens: 20, OutputTokens: 3}))
	// An ended call stays on screen, finished, until the saved turn replaces it.
	if a.st.RunTurns != 2 || a.st.RunJS != 1 || len(a.st.LiveCalls) != 1 || !a.st.LiveCalls[0].Ended || a.st.LiveCalls[0].Running {
		t.Fatalf("turns %d js %d live %+v", a.st.RunTurns, a.st.RunJS, a.st.LiveCalls)
	}
	a.applyWire(rt, wireEvent(api.EventDone, api.DoneData{}))
	if a.st.Running {
		t.Fatal("still running after done")
	}
	if len(a.st.LiveCalls) != 0 {
		t.Fatalf("an ended call outlived the run: %+v", a.st.LiveCalls)
	}
}

func TestLiveCallShowsItsTimer(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "go"})
	a.st.Running, a.st.Phase = true, "running"
	a.st.LiveCalls = []liveCall{{ID: "c1", Code: "sh('sleep 9')", Running: true, Started: time.Now().Add(-12 * time.Second)}}
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "12s") {
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
	timer := regexp.MustCompile(`\b[0-9]+(?:\.[0-9])?s\b`)
	if text := screenText(s); !timer.MatchString(text) || strings.Contains(text, timer.FindString(text)+"/") {
		t.Fatalf("%s", text)
	}
}
