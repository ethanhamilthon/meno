package agent

import (
	"os"
	"testing"

	"meno/internal/llm"
)

// A fork carries the turns up to its boundary and records where it came from.
func TestForkKeepsTurnsUpToTheBoundary(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "start"})
	s.Append(Turn{Role: llm.RoleAssistant, Text: "call", CallID: "c1", Code: "x", Err: "boom"})
	s.Append(Turn{Role: llm.RoleAssistant, Text: "call2", CallID: "c2", Code: "y"})
	s.Append(Turn{Role: llm.RoleAssistant, Text: "done"})
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	for hid := int64(1); hid <= 4; hid++ {
		f, warn, err := ForkSession(home, s.ID, hid, "")
		if err != nil || warn != "" {
			t.Fatalf("fork at %d: %v %q", hid, err, warn)
		}
		if f.ForkSource != s.ID || f.ForkBoundary != hid || len(f.Snapshot()) != int(hid) {
			t.Fatalf("fork at %d: %+v", hid, f)
		}
	}
	if len(s.Snapshot()) != 4 {
		t.Fatal("the source changed")
	}
}

// A fork before a compaction must not carry the summary that came out of it,
// and must keep replay groups whole.
func TestForkNeverUsesFutureSummary(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	appendN(s, 4)
	s.Append(Turn{Role: llm.RoleAssistant, Text: "call", CallID: "c", Code: "x", ReplayGroup: "g", Replay: &llm.Msg{Role: llm.RoleAssistant}}) // 5
	s.Append(Turn{Role: llm.RoleAssistant, Text: "result", ReplayGroup: "g"})                                                                  // 6
	appendN(s, 4)                                                                                                                              // 7..10
	all := s.Snapshot()
	s.ensureLog().Compact(all[:6], all[6:], SummaryRev{Rev: 1, Summary: "future facts"})
	s.Summary = "future facts"
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	early, _, err := ForkSession(home, s.ID, 3, "")
	if err != nil || early.Summary != "" || len(early.Snapshot()) != 3 {
		t.Fatalf("early fork: %+v %v", early, err)
	}
	mid, _, err := ForkSession(home, s.ID, 5, "") // inside a replay group: expands to 6
	if err != nil || mid.ForkBoundary != 6 || mid.Summary != "future facts" || len(mid.Snapshot()) != 0 {
		t.Fatalf("mid fork: %+v %v", mid, err)
	}
	if err := SaveSession(home, mid); err != nil {
		t.Fatal(err)
	}
	full, _ := ReadHistory(home, mid.ID, 0, 100)
	if len(full) != 6 {
		t.Fatalf("fork history has %d turns, want 6", len(full))
	}
	late, _, err := ForkSession(home, s.ID, 9, "")
	if err != nil || late.Summary != "future facts" || len(late.Snapshot()) != 3 {
		t.Fatalf("late fork: %+v %v", late, err)
	}
}

func TestForkMissingCwdAndLegacyWarning(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	gone := t.TempDir()
	s := NewSession(gone, "m")
	appendN(s, 2)
	s.Legacy = true
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(gone)
	if _, _, err := ForkSession(home, s.ID, 2, ""); err == nil {
		t.Fatal("a missing cwd must require an explicit directory")
	}
	f, warn, err := ForkSession(home, s.ID, 2, t.TempDir())
	if err != nil || warn != LegacyForkWarning || f.Legacy {
		t.Fatalf("%+v %q %v", f, warn, err)
	}
}
