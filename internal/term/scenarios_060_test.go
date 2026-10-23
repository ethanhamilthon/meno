//go:build termtest

package term

import (
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
