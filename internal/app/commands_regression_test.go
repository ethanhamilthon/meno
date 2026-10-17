package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/db"
	"meno/internal/llm"
)

func TestClearRemovesPersistedContextPreservesAccounting(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	id := a.st.Session.ID
	a.st.Session.Summary = "old compacted context"
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "old turn"})
	a.st.Usage = llm.Usage{In: 123, Out: 45, Cached: 6}
	a.st.Spent = 1.25
	a.st.Unpriced = true
	a.st.Context = 168
	a.st.Scroll = 9
	a.saveSession()
	a.runCommand("clear")
	if a.st.Session.ID != id || a.st.Session.Summary != "" || len(a.st.Session.ToMsgs()) != 0 || a.st.Context != 0 || a.st.Scroll != 0 {
		t.Fatal("clear retained context or replaced session")
	}
	saved, err := agent.LoadSession(a.paths.Home, id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Summary != "" || len(saved.Snapshot()) != 0 || saved.Metrics.Context != 0 {
		t.Fatalf("persisted stale context: %+v", saved)
	}
	if saved.Metrics.Usage != a.st.Usage || saved.Metrics.Usage.In != 123 || saved.Metrics.Spent != 1.25 || !saved.Metrics.Unpriced {
		t.Fatalf("lost accounting: %+v", saved.Metrics)
	}
	a.runCommand("new")
	if a.st.Usage != (llm.Usage{}) || a.st.Spent != 0 || a.st.Unpriced || a.st.Context != 0 {
		t.Fatal("new retained accounting")
	}
}

func TestResumeCommandReportsListingFailure(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	db.CloseAll()
	_ = os.Remove(filepath.Join(a.paths.Home, "meno.db-wal"))
	_ = os.Remove(filepath.Join(a.paths.Home, "meno.db-shm"))
	if err := os.WriteFile(filepath.Join(a.paths.Home, "meno.db"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.runCommand("resume")
	if !strings.Contains(a.lastNotice(), "resume failed:") || a.st.Over == OverResume {
		t.Fatalf("silenced listing failure: %q", a.lastNotice())
	}
}
