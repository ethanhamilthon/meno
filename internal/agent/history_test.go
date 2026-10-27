package agent

import (
	"testing"

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
