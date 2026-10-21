package agent

import "testing"

// A turn appended while a summary is being generated must survive the commit.
func TestCompactKeepsTurnsAppendedMeanwhile(t *testing.T) {
	l := newSafeLog(nil, 0)
	for i := 0; i < 6; i++ {
		l.Append(Turn{Role: "user", Text: "t"})
	}
	all := l.Snapshot()
	head, tail := all[:2], all[2:]
	l.Append(Turn{Role: "assistant", Text: "late"}) // arrives during the provider call
	l.Compact(head, tail, SummaryRev{Summary: "s"})
	got := l.Snapshot()
	if len(got) != 5 || got[len(got)-1].Text != "late" {
		t.Fatalf("active turns after compact: %d, last %q", len(got), got[len(got)-1].Text)
	}
	if len(l.Pending().Archive) != 2 {
		t.Fatalf("archive %d", len(l.Pending().Archive))
	}
}
