package agent

import (
	"os"
	"testing"

	"meno/internal/llm"
)

func entriesMap(t *testing.T, kv *KV, boundary int64) map[string]string {
	t.Helper()
	es, err := kv.EntriesAt(boundary)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range es {
		m[e.Key] = e.JSON
	}
	return m
}

// A fork sees the state of its boundary: a write made by the step that the
// boundary ends is included, a later write is not, and a failed step's earlier
// writes survive.
func TestForkReconstructsStateAtBoundary(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	kv, err := OpenKV(home, s.ID, NSState)
	if err != nil {
		t.Fatal(err)
	}
	s.Append(Turn{Role: llm.RoleUser, Text: "start"}) // 1
	// Step 2 sets a, then fails: the call turn still records it.
	if err := kv.Set("a", "1", s.ExecBoundary()); err != nil {
		t.Fatal(err)
	}
	s.Append(Turn{Role: llm.RoleAssistant, Text: "call", CallID: "c1", Code: "x", Err: "boom"}) // 2
	// Step 3 changes a and adds b.
	if err := kv.Set("a", "2", s.ExecBoundary()); err != nil {
		t.Fatal(err)
	}
	if err := kv.Set("b", "true", s.ExecBoundary()); err != nil {
		t.Fatal(err)
	}
	s.Append(Turn{Role: llm.RoleAssistant, Text: "call2", CallID: "c2", Code: "y"}) // 3
	if _, err := kv.Delete("b", s.ExecBoundary()); err != nil {
		t.Fatal(err)
	}
	s.Append(Turn{Role: llm.RoleAssistant, Text: "done"}) // 4
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	for hid, want := range map[int64]string{1: "", 2: "a=1", 3: "a=2,b=true", 4: "a=2"} {
		f, warn, err := ForkSession(home, s.ID, hid, "")
		if err != nil || warn != "" {
			t.Fatalf("fork at %d: %v %q", hid, err, warn)
		}
		fkv, _ := OpenKV(home, f.ID, NSState)
		got := ""
		es, _ := fkv.Entries()
		for i, e := range es {
			if i > 0 {
				got += ","
			}
			got += e.Key + "=" + e.JSON
		}
		if got != want {
			t.Fatalf("fork at %d state %q, want %q", hid, got, want)
		}
		if f.ForkSource != s.ID || f.ForkBoundary != hid || len(f.Snapshot()) != int(hid) {
			t.Fatalf("fork at %d: %+v", hid, f)
		}
	}
	// Independence: writing to a fork leaves the source alone.
	f, _, _ := ForkSession(home, s.ID, 4, "")
	fkv, _ := OpenKV(home, f.ID, NSState)
	_ = fkv.Set("only", "1", 5)
	if m := entriesMap(t, kv, 99); len(m) != 1 || m["a"] != "2" {
		t.Fatalf("source changed: %v", m)
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

func TestStateLimitsAndAtomicFailure(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	kv, _ := OpenKV(home, "s", NSState)
	kv.MaxValue, kv.MaxTotal = 10, 15
	if err := kv.Set("a", "123456789", 1); err != nil {
		t.Fatal(err)
	}
	if err := kv.Set("b", "12345678901", 1); err == nil {
		t.Fatal("over the per-value limit")
	}
	if err := kv.Set("b", "1234567", 1); err == nil {
		t.Fatal("over the total limit")
	}
	if err := kv.Set("a", "1234567890", 1); err != nil { // replaces, so the total is 10
		t.Fatal(err)
	}
	if v, ok, _ := kv.Get("a"); !ok || v != "1234567890" {
		t.Fatal(v, ok)
	}
}
