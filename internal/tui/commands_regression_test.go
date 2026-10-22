package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/db"
	"meno/internal/llm"
)

func TestNewStartsWithoutAccounting(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Usage = llm.Usage{In: 123, Out: 45, Cached: 6}
	a.st.Spent = 1.25
	a.st.Unpriced = true
	a.st.Context = 168
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "old turn"})
	a.runCommand("new")
	if a.st.Usage != (llm.Usage{}) || a.st.Spent != 0 || a.st.Unpriced || a.st.Context != 0 || a.st.Session.Len() != 0 {
		t.Fatal("new retained accounting")
	}
}

func TestRemovedCommandsAreUnknown(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for _, cmd := range []string{"clear", "resume", "cache", "notify", "attach", "theme"} {
		a.st.Notices = nil
		a.runCommand(cmd)
		if !strings.Contains(a.lastNotice(), "unknown command") {
			t.Fatalf(":%s must be gone, notice %q", cmd, a.lastNotice())
		}
	}
	for _, c := range commandList {
		switch c.name {
		case "/clear", "/resume", "/cache", "/notify", "/attach", "/theme":
			t.Fatalf("%s still in the command list", c.name)
		}
	}
}

func TestSessionsCommandReportsListingFailure(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	// Quiesce hydration before deliberately replacing the database. Otherwise a
	// concurrent initial read can reopen it between CloseAll and corruption.
	a.globalWatch.stop()
	a.st.Runtime.watch()
	a.watchers.Wait()
	pumpFor(a, 20*time.Millisecond)
	a.running.Wait()
	db.CloseAll()
	_ = os.Remove(filepath.Join(a.paths.Home, "meno.db-wal"))
	_ = os.Remove(filepath.Join(a.paths.Home, "meno.db-shm"))
	if err := os.WriteFile(filepath.Join(a.paths.Home, "meno.db"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.runCommand("sessions")
	if a.st.Over != OverSessions {
		t.Fatal("the dialog opens at once, before the list is loaded")
	}
	pump(t, a, func() bool { return a.lastNotice() != "" })
	if !strings.Contains(a.lastNotice(), "sessions failed:") {
		t.Fatalf("silenced listing failure: %q", a.lastNotice())
	}
}
