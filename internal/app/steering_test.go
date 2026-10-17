package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

func TestSteeringQueuesWhileRunningAndRecallsWithUp(t *testing.T) {
	cl := &scriptedClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	release := make(chan struct{})
	a.prepareHook = func() { <-release }
	typeRunes(a, "first")
	a.onKey(keyK(tcell.KeyEnter))
	if !a.st.Running {
		t.Fatal("turn did not start")
	}
	typeRunes(a, "second thought")
	a.onKey(keyK(tcell.KeyEnter))
	if len(a.st.Input) != 0 || a.st.Queue.Len() != 1 {
		t.Fatalf("input=%q queued=%v", string(a.st.Input), a.st.Queue.Items())
	}
	a.draw()
	if out := screenText(scr); !strings.Contains(out, "queued") || !strings.Contains(out, "second thought") {
		t.Fatalf("queue panel missing:\n%s", out)
	}
	a.onKey(keyK(tcell.KeyUp))
	if string(a.st.Input) != "second thought" || a.st.Queue.Len() != 0 {
		t.Fatalf("recall: input=%q queued=%v", string(a.st.Input), a.st.Queue.Items())
	}
	a.abortTurn()
	close(release)
	pump(t, a, func() bool { return !a.st.Running })
}

func TestSteeringDeliveredToModelAndCtrlCReturnsQueue(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	release := make(chan struct{})
	a.prepareHook = func() { <-release }
	typeRunes(a, "first")
	a.onKey(keyK(tcell.KeyEnter))
	typeRunes(a, "second")
	a.onKey(keyK(tcell.KeyEnter))
	close(release)
	pump(t, a, func() bool { return !a.st.Running })
	var users []string
	for _, tr := range a.st.Session.Snapshot() {
		if tr.Role == llm.RoleUser {
			users = append(users, tr.Text)
		}
	}
	if len(users) != 2 || users[0] != "first" || users[1] != "second" || a.st.Queue.Len() != 0 {
		t.Fatalf("user turns: %q", users)
	}

	// Ctrl+C: what was not delivered yet comes back into the composer.
	release2 := make(chan struct{})
	a.prepareHook = func() { <-release2 }
	typeRunes(a, "third")
	a.onKey(keyK(tcell.KeyEnter))
	typeRunes(a, "fourth")
	a.onKey(keyK(tcell.KeyEnter))
	a.onKey(keyK(tcell.KeyCtrlC))
	close(release2)
	pump(t, a, func() bool { return !a.st.Running })
	if string(a.st.Input) != "fourth" || a.st.Queue.Len() != 0 {
		t.Fatalf("after ctrl+c: input=%q queued=%v", string(a.st.Input), a.st.Queue.Items())
	}
}
