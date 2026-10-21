//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

// The first-run screen promises that ctrl+c quits.
func TestFirstRunCtrlCQuits(t *testing.T) {
	s := start(t, options{firstRun: true})
	s.Send(CtrlC)
	done := make(chan error, 1)
	go func() { done <- s.WaitExit() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("ctrl+c did not quit:\n%s", s.Screen())
	}
}

// /q only detaches: the run goes on in the daemon, and a second window sees it.
func TestQuitDetachesAndTheRunSurvives(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		q.TextPartial("long answer start ")
		time.Sleep(3 * time.Second)
		q.Text("and the end-marker")
	}
	home := t.TempDir()
	a := start(t, options{fake: f, home: home, cols: 110, rows: 28})
	a.Say("go")
	a.WaitFor("long answer start")
	a.Cmd("q")
	if err := a.WaitExit(); err != nil {
		t.Logf("exit: %v", err)
	}
	b := start(t, options{fake: f, home: home, cols: 110, rows: 28})
	b.Cmd("sessions")
	b.WaitFor("running")
}

// An unsent draft is on disk when the window is killed, and /drafts brings it
// back in the next window without a session switch.
func TestDraftSurvivesAKilledWindow(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("ok") }
	home := t.TempDir()
	a := start(t, options{fake: f, home: home, cols: 100, rows: 24})
	a.Type("keep this half-written thought")
	a.WaitFor("keep this half-written thought")
	time.Sleep(900 * time.Millisecond) // a tick writes the draft
	_ = a.cmd.Process.Kill()
	<-a.exited
	b := start(t, options{fake: f, home: home, cols: 100, rows: 24})
	b.WaitFor("unsent draft")
	b.Cmd("drafts")
	screen := b.WaitFor("keep this half-written thought")
	if strings.Contains(screen, "half-written thought half-written") {
		t.Fatalf("draft duplicated:\n%s", screen)
	}
}

// /stop-all asks before it stops anything.
func TestStopAllNeedsConfirmation(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		q.TextPartial("working ")
		time.Sleep(4 * time.Second)
		q.Text("finished-marker")
	}
	s := start(t, options{fake: f, cols: 110, rows: 28})
	s.Say("go")
	s.WaitFor("working")
	s.Cmd("stop-all")
	s.WaitFor("repeat /stop-all")
	s.WaitFor("finished-marker") // still running: the first command only asked
}
