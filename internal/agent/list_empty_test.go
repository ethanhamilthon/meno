package agent

import (
	"testing"

	"meno/internal/llm"
)

// A session nobody wrote to (older versions stored one at every start) is not
// listed; one with a message, a summary or background work is.
func TestListHidesEmptySessions(t *testing.T) {
	dir := t.TempDir()
	empty := NewSession("/x", "m")
	if err := SaveSession(dir, empty); err != nil {
		t.Fatal(err)
	}
	used := NewSession("/x", "m")
	used.Append(Turn{Role: llm.RoleUser, Text: "hi"})
	if err := SaveSession(dir, used); err != nil {
		t.Fatal(err)
	}
	sum := NewSession("/x", "m")
	sum.Summary = "earlier work"
	if err := SaveSession(dir, sum); err != nil {
		t.Fatal(err)
	}
	rows, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.ID] = true
	}
	if got[empty.ID] || !got[used.ID] || !got[sum.ID] || len(rows) != 2 {
		t.Fatalf("listed %v", got)
	}
}
