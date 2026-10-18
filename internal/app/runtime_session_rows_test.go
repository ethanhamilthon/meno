package app

import (
	"testing"
	"time"

	"meno/internal/agent"
)

func TestSessionRowsMergeMemoryAndDatabase(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "stored one")
	id := a.st.Session.ID
	a.runCommand("new")
	a.dbRows, _ = agent.ListRecentSessions(a.paths.Home, 200)
	rows := a.sessionRows()
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if r.ID == id && (r.rt == nil || r.Title != "stored one" || r.Turns != 2) {
			t.Fatalf("row %+v", r)
		}
	}
	// Dropping the runtime leaves the database row.
	a.dropRuntime(a.rts[id])
	rows = a.sessionRows()
	found := false
	for _, r := range rows {
		if r.ID == id {
			found = r.rt == nil && r.Title == "stored one"
		}
	}
	if !found {
		t.Fatalf("rows %+v", rows)
	}
}

func TestEmptyNewSessionIsNotKeptWhenLeft(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	say(t, a, "work")
	a.runCommand("new")
	a.runCommand("new")
	a.runCommand("new")
	if len(a.rtList) != 2 {
		t.Fatalf("%d runtimes: empty unused sessions pile up", len(a.rtList))
	}
	a.runCommand("quit")
	a.Close()
	list, _ := agent.ListSessions(a.paths.Home)
	if len(list) != 1 {
		t.Fatalf("empty sessions must not be stored, got %d", len(list))
	}
}

var _ = time.Second
