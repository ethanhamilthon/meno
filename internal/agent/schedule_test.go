package agent

import (
	"testing"

	"meno/internal/llm"
)

func TestScheduledCompactionKeepsGroupsAndHistory(t *testing.T) {
	s := NewSession("/w", "m")
	appendN(s, 6)
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "a", Code: "x", ReplayGroup: "g", Replay: &llm.Msg{}})
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "b", Code: "y", ReplayGroup: "g"}) // the batch that called session.compact
	if err := s.ScheduleCompaction("sum", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.ScheduleCompaction("again", 0); err != ErrAlreadyScheduled {
		t.Fatalf("a second schedule in one batch must fail: %v", err)
	}
	kept, ok := s.ApplyScheduled()
	if !ok || kept != 2 || s.Summary != "sum" {
		t.Fatalf("keep=0 still retains the current group: kept=%d ok=%v summary=%q", kept, ok, s.Summary)
	}
	if h := s.Snapshot()[0].HID; h != 7 {
		t.Fatalf("active context starts at %d", h)
	}
	p := s.ensureLog().Pending()
	if len(p.Archive) != 6 || len(p.Summaries) != 1 {
		t.Fatalf("pending: %+v", p)
	}
	// N larger than the context keeps everything; N inside a group expands it.
	s2 := NewSession("/w", "m")
	appendN(s2, 2)
	s2.Append(Turn{Role: llm.RoleAssistant, CallID: "a", ReplayGroup: "g", Replay: &llm.Msg{}})
	s2.Append(Turn{Role: llm.RoleAssistant, CallID: "b", ReplayGroup: "g"})
	appendN(s2, 2)
	s2.Append(Turn{Role: llm.RoleAssistant, CallID: "c", ReplayGroup: "h", Replay: &llm.Msg{}})
	_ = s2.ScheduleCompaction("s", 4)
	kept, _ = s2.ApplyScheduled()
	if kept != 5 { // 4 requested, group g cannot be split: 2 + 1 + 2... counted from the cut
		t.Fatalf("kept %d", kept)
	}
	if err := s.ScheduleCompaction("", 1); err == nil {
		t.Fatal("empty summary")
	}
}

func TestNormalizeTitle(t *testing.T) {
	for in, want := range map[string]string{"  Fix login bug ": "Fix login bug", "one": "one"} {
		if got, err := NormalizeTitle(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "  ", "a\nb", "a b c d e f g", "tab\there"} {
		if _, err := NormalizeTitle(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}
