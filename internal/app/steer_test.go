package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
)

// An event for an idle session wakes it with a continuation that carries the
// event; the same event ID is never delivered twice.
func TestIdleSessionWakesOnEventOnce(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "start")
	ev := agent.Event{ID: "e1", Session: a.st.Session.ID, Source: agent.SourceTask, From: "t1", Kind: "task_done", Owner: a.st.RequestID, Text: "build ok"}
	a.enqueueEvent(ev)
	pump(t, a, func() bool { return len(cl.requests()) == 2 && !a.st.Running })
	if got := lastUser(cl.requests()[1]); !strings.Contains(got, "build ok") || !strings.Contains(got, "source=\"task\"") {
		t.Fatalf("continuation request: %q", got)
	}
	a.enqueueEvent(ev) // redelivery
	pump(t, a, func() bool { return true })
	if len(cl.requests()) != 2 {
		t.Fatalf("a redelivered event started another loop: %d requests", len(cl.requests()))
	}
}

// Events of a cancelled request never wake the session.
func TestCancelledRequestDoesNotAutoResume(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "start")
	owner := a.st.RequestID
	a.cancelRequest()
	a.enqueueEvent(agent.Event{ID: "e2", Session: a.st.Session.ID, Source: agent.SourceTask, Owner: owner, Text: "late"})
	pump(t, a, func() bool { return true })
	if len(cl.requests()) != 1 || a.st.Running {
		t.Fatalf("a cancelled request resumed: %d requests", len(cl.requests()))
	}
	// A different request's event still wakes it.
	a.enqueueEvent(agent.Event{ID: "e3", Session: a.st.Session.ID, Source: agent.SourceTask, Owner: "other", Text: "other work done"})
	pump(t, a, func() bool { return len(cl.requests()) == 2 && !a.st.Running })
	if a.st.RequestID != "other" {
		t.Fatalf("a continuation uses the event's request, got %q", a.st.RequestID)
	}
}
