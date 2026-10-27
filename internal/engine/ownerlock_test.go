package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/db"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/mcp"
)

// A stored session has one owner process: a second engine on the same home is
// refused until the first one lets go.
func TestStoredSessionHasOneOwner(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	id := newSession(t, e)
	send(t, e, id) // stored by its first message

	other, err := New(Options{MCP: mcp.NewManager(), Home: e.home})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_, err = other.Snapshot(context.Background(), id)
	if apiCode(err) != api.CodeConflict || !strings.Contains(err.Error(), "another meno process") {
		t.Fatalf("a second owner must be refused: %v", err)
	}

	e.Close() // the first owner exits: the lock goes with it
	if _, err = other.Snapshot(context.Background(), id); err != nil {
		t.Fatalf("after the owner closed: %v", err)
	}
}

func TestDeletingASessionReleasesItsLock(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	id := newSession(t, e)
	send(t, e, id)
	if err := e.DeleteSession(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	e.lockMu.Lock()
	held := len(e.locks)
	e.lockMu.Unlock()
	if held != 0 {
		t.Fatalf("%d locks held after delete", held)
	}
}

// Another process starting up, or failing to open a session it does not own,
// must not touch the tasks or the transcript of the owner.
func TestStartingASecondEngineLeavesTheOwnersTasksAlone(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	id := newSession(t, e)
	send(t, e, id)
	d, err := db.Open(e.home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec("INSERT INTO tasks(id,session_id,owner,name,command,cwd,status,notify,timeout_ms,started_at) VALUES('live',?,'','','sleep','/tmp','running',0,0,'')", id); err != nil {
		t.Fatal(err)
	}
	before, _ := agent.LoadSession(e.home, id)

	other, err := New(Options{MCP: mcp.NewManager(), Home: e.home})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.Snapshot(context.Background(), id); apiCode(err) != api.CodeConflict {
		t.Fatalf("the second engine must be refused: %v", err)
	}
	var status string
	_ = d.QueryRow("SELECT status FROM tasks WHERE id='live'").Scan(&status)
	after, _ := agent.LoadSession(e.home, id)
	if status != "running" || after.Len() != before.Len() {
		t.Fatalf("the owner's state changed: task %q, turns %d -> %d", status, before.Len(), after.Len())
	}
}
