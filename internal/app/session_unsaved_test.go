package app

import (
	"context"
	"github.com/gdamore/tcell/v2"
	"strings"
	"testing"

	"meno/internal/llm"
	"meno/internal/proto"
)

func daemonSessions(t *testing.T, a *App) []proto.SessionItem {
	t.Helper()
	l, err := a.cl.ListSessions(context.Background(), proto.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return l.Sessions
}

// Opening meno and starting new sessions stores nothing: a session exists for
// the list only once a message was sent in it.
func TestEmptySessionsAreNotListed(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	first := a.st.Runtime.ID()
	a.newSession()
	a.newSession()
	if n := len(a.rtList); n != 1 {
		t.Fatalf("%d runtimes follow unused sessions", n)
	}
	if n := len(daemonSessions(t, a)); n != 0 {
		t.Fatalf("daemon lists %d empty sessions", n)
	}
	for _, r := range a.sessionRows() {
		t.Fatalf("empty session shown in /sessions: %+v", r)
	}
	say(t, a, "first words")
	rows := daemonSessions(t, a)
	if len(rows) != 1 || rows[0].ID != a.st.Runtime.ID() || rows[0].ID == first {
		t.Fatalf("after the first message: %+v", rows)
	}
}

// The first message names the session at once, not when the run ends.
func TestSessionIsNamedWhenTheFirstMessageIsSent(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	typeRunes(a, "plan the migration")
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return a.st.Session.Title != "" })
	if got := a.st.Session.DisplayTitle(); !strings.Contains(got, "plan the migration") {
		t.Fatalf("title %q", got)
	}
}

// A daemon that restarted forgot the empty session; the text still gets sent.
func TestMessageSurvivesDaemonForgettingTheSession(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	old := a.st.Runtime.ID()
	if err := a.cl.DeleteSession(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	say(t, a, "still here")
	if a.st.Runtime.ID() == old {
		t.Fatal("the lost session was kept")
	}
	if got := daemonSessions(t, a); len(got) != 1 || got[0].ID != a.st.Runtime.ID() {
		t.Fatalf("sessions %+v", got)
	}
}
