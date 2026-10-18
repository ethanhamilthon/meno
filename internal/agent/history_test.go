package agent

import (
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
)

func appendN(s *Session, n int) {
	for i := 0; i < n; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: string(rune('a' + i))})
	}
}

// Compaction moves turns out of the active context but keeps every original,
// with stable IDs, across repeated compactions and reloads.
func TestCompactionKeepsFullHistory(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/w", "m")
	appendN(s, 10)
	for round := 0; round < 2; round++ {
		all := s.Snapshot()
		cut := len(all) - 4
		s.ensureLog().Compact(all[:cut], all[cut:], SummaryRev{Rev: round + 1, Summary: "sum"})
		s.Summary = "sum"
		if err := SaveSession(home, s); err != nil {
			t.Fatal(err)
		}
		appendN(s, 3)
	}
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHistory(home, s.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 16 {
		t.Fatalf("history has %d turns, want 16", len(got))
	}
	for i, tr := range got {
		if tr.HID != int64(i+1) {
			t.Fatalf("turn %d has HID %d", i, tr.HID)
		}
	}
	loaded, _ := LoadSession(home, s.ID)
	if len(loaded.Snapshot()) != 7 || loaded.Snapshot()[0].HID != 10 {
		t.Fatalf("active context: %+v", loaded.Snapshot())
	}
	loaded.Append(Turn{Role: llm.RoleUser, Text: "next"})
	if h := loaded.Snapshot()[len(loaded.Snapshot())-1].HID; h != 17 {
		t.Fatalf("a new turn after reload got HID %d, want 17", h)
	}
	page, _ := ReadHistory(home, s.ID, 12, 3)
	if len(page) != 3 || page[0].HID != 13 {
		t.Fatalf("pagination: %+v", page)
	}
}

// A database written before history IDs existed upgrades without loss, twice.
func TestUpgradeFromV1KeepsLegacySessions(t *testing.T) {
	home := t.TempDir()
	d, err := db.OpenOld(home, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', cwd TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', metrics TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL)",
		"CREATE TABLE turns (session_id TEXT NOT NULL, seq INTEGER NOT NULL, role TEXT NOT NULL, text TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '', call_id TEXT NOT NULL DEFAULT '', code TEXT NOT NULL DEFAULT '', output TEXT NOT NULL DEFAULT '', err TEXT NOT NULL DEFAULT '', replay TEXT, replay_group TEXT NOT NULL DEFAULT '', time TEXT NOT NULL, PRIMARY KEY (session_id, seq)) WITHOUT ROWID",
		"INSERT INTO sessions(id,summary,updated_at) VALUES('s1','old summary','2026-01-01T00:00:00Z')",
		"INSERT INTO turns(session_id,seq,role,text,time) VALUES('s1',0,'user','one','2026-01-01T00:00:00Z'),('s1',1,'assistant','two','2026-01-01T00:00:00Z')",
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	d.Close()
	for i := 0; i < 2; i++ {
		s, err := LoadSession(home, "s1")
		if err != nil {
			t.Fatal(err)
		}
		if !s.Legacy || s.Coverage != CoveragePartial || len(s.Snapshot()) != 2 || s.Snapshot()[1].HID != 2 {
			t.Fatalf("round %d: %+v", i, s)
		}
		if err := SaveSession(home, s); err != nil {
			t.Fatal(err)
		}
		CloseSessionStores()
	}
}
