package app

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/proto"
	"meno/internal/ui"
	"strings"
	"testing"
)

func TestGlobalCountsUseExclusiveAskingAndThemeColors(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.globalTasks = 2
	a.globalSessions = []proto.SessionItem{{Running: true}, {Running: true, NeedsYou: true}, {Completed: 2, ReadThrough: 1}}
	segments := a.globalCounts()
	text := joinSegText(segments)
	if text != "1 running | 2 tasks | 1 Asking | 1 done" {
		t.Fatal(text)
	}
	if segments[0].FG != ui.Green || segments[2].FG != ui.Purple || segments[4].FG != ui.Orange || segments[6].FG != ui.Blue {
		t.Fatal("counter colors")
	}
}

func TestContextOpensAndReturnsToCommand(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Mode = ui.ModeCommand
	a.runCommand("context")
	a.draw()
	if a.st.Over != OverContext || !strings.Contains(screenText(s), "Session total") {
		t.Fatal(screenText(s))
	}

	a.closeOverlay()
	if a.st.Mode != ui.ModeCommand {
		t.Fatal("context return")
	}
}

func TestUnreadSurvivesModalAndScrollUntilSummaryVisible(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	id := a.st.Session.ID()
	a.st.Session.Completed = 1
	a.st.Session.ReadThrough = 0
	for i := 0; i < 50; i++ {
		a.st.Session.Append(agent.Turn{Role: "user", Text: "history row"})
	}
	a.st.Session.Append(agent.Turn{Role: "assistant", Text: "result", Run: &agent.RunStats{Status: agent.RunOK}})
	a.openOverlay(OverHelp)
	a.draw()
	a.closeOverlay()
	a.ackPending = false
	a.st.Scroll = 10
	a.draw()
	if a.ackPending {
		t.Fatal("scrolled result acknowledged")
	}
	a.st.Scroll = 0
	a.openOverlay(OverHelp)
	a.draw()
	if a.ackPending {
		t.Fatal("covered result acknowledged")
	}
	a.closeOverlay()
	a.draw()
	if !a.ackPending {
		t.Fatal("visible result not acknowledged", id)
	}
}

func TestWireDoesNotStealCommandFocus(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Mode = ui.ModeCommand
	a.inRT(a.st.Session.ID(), func() { a.st.Phase = "thinking" })
	if a.st.Mode != ui.ModeCommand {
		t.Fatal("event stole command focus")
	}
}

func TestCommandUndoDoesNotChangeMessage(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	typeRunes(a, "message")
	a.onKey(keyK(tcell.KeyEsc))
	typeRunes(a, "settings")
	a.onKey(keyK(tcell.KeyCtrlZ))
	if string(a.st.Cmd) != "setting" {
		t.Fatal(string(a.st.Cmd))
	}
	a.onKey(keyK(tcell.KeyEsc))
	if string(a.st.Input) != "message" {
		t.Fatal("message draft affected")
	}
}
