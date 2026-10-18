package agent

import (
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
)

// A meno.db written before system events has a turns table without the column.
func TestSystemColumnAddedToExistingTurnsTable(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	d, err := db.OpenOld(home, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', cwd TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', metrics TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL)",
		"CREATE TABLE turns (session_id TEXT NOT NULL, seq INTEGER NOT NULL, role TEXT NOT NULL, text TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '', call_id TEXT NOT NULL DEFAULT '', code TEXT NOT NULL DEFAULT '', output TEXT NOT NULL DEFAULT '', err TEXT NOT NULL DEFAULT '', replay TEXT, replay_group TEXT NOT NULL DEFAULT '', time TEXT NOT NULL, PRIMARY KEY (session_id, seq)) WITHOUT ROWID",
		"INSERT INTO sessions(id,title,updated_at) VALUES('s1','t','2026-01-01T00:00:00Z')",
		"INSERT INTO turns(session_id,seq,role,text,time) VALUES('s1',0,'user','old text','2026-01-01T00:00:00Z')",
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	d.Close()
	s, err := LoadSession(home, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot()[0]; got.Text != "old text" || got.System != "" {
		t.Fatalf("%+v", got)
	}
	if s.Provider != "" || s.Effort != "" {
		t.Fatalf("old session has no provider/effort: %+v", s)
	}
	s.Provider, s.Effort = "p", "high"
	s.Append(Turn{Role: llm.RoleUser, Text: "new", System: "<system-event>e</system-event>"})
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	s, _ = LoadSession(home, "s1")
	if s.Provider != "p" || s.Effort != "high" {
		t.Fatalf("%+v", s)
	}
	if s.Snapshot()[1].System != "<system-event>e</system-event>" {
		t.Fatalf("%+v", s.Snapshot())
	}
}
