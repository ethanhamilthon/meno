package agent

import (
	"meno/internal/llm"
	"testing"
)

func TestSaveCompletionCountsOnlyWhenRequested(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/work", "model")
	s.Append(Turn{Role: llm.RoleUser, Text: "hello"})
	for i, count := range []bool{false, true, false, true} {
		if err := SaveCompletion(home, s, count); err != nil {
			t.Fatal(err)
		}
		got, err := LoadSession(home, s.ID)
		if err != nil || got.Len() != 1 {
			t.Fatalf("history: %v", err)
		}
		d, err := openDB(home)
		if err != nil {
			t.Fatal(err)
		}
		var rows, completed int
		if err = d.QueryRow("SELECT count(*),coalesce(sum(completed),0) FROM session_attention WHERE session_id=?", s.ID).Scan(&rows, &completed); err != nil {
			t.Fatal(err)
		}
		want := []int{0, 1, 1, 2}[i]
		if completed != want || (i == 0 && rows != 0) {
			t.Fatalf("rows=%d completed=%d, want=%d", rows, completed, want)
		}
	}
}
