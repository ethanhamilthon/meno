package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"meno/internal/llm"
	"meno/internal/store"
)

func TestSQLiteRoundTripAndRewrite(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/work", "m1")
	cost := 0.5
	s.Append(Turn{Role: llm.RoleUser, Text: "hi \U0001F600\nline2", Time: time.Now()})
	s.Append(Turn{Role: llm.RoleAssistant, Text: "run", CallID: "c1", Code: "1+1", Output: "2", Replay: &llm.Msg{Role: llm.RoleAssistant, Thought: "t"}, ReplayGroup: "g"})
	s.Requests = []RequestUsage{{Provider: "p", Model: "m", Usage: llm.Usage{In: 10, Out: 2}, Cost: &cost}, {Provider: "p", Model: "m"}}
	s.Metrics = Metrics{Spent: 1.5, Context: 12, Unpriced: true}
	s.Summary = "sum"
	s.Provider, s.Effort, s.Tool = "prov", "low", "bash"
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "prov" || got.Model != "m1" || got.Effort != "low" || got.Tool != "bash" {
		t.Fatalf("model of the session: %q %q %q", got.Provider, got.Model, got.Effort)
	}
	turns := got.Snapshot()
	if len(turns) != 2 || turns[0].Text != "hi \U0001F600\nline2" || turns[1].Replay == nil || turns[1].Replay.Thought != "t" || turns[1].ReplayGroup != "g" {
		t.Fatalf("turns %+v", turns)
	}
	if got.Summary != "sum" || got.Metrics.Spent != 1.5 || !got.Metrics.Unpriced || got.Title != "hi \U0001F600 line2" {
		t.Fatalf("session %+v", got)
	}
	if len(got.Requests) != 2 || got.Requests[0].Cost == nil || *got.Requests[0].Cost != 0.5 || got.Requests[1].Cost != nil || got.Requests[0].Usage.In != 10 {
		t.Fatalf("requests %+v", got.Requests)
	}
	// Rewriting with fewer turns (compaction) must drop old rows.
	s.Replace(nil)
	s.Requests = nil
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, _ = LoadSession(dir, s.ID)
	if len(got.Snapshot()) != 0 || len(got.Requests) != 0 {
		t.Fatal("stale rows survived rewrite")
	}
	list, err := ListSessions(dir)
	if err != nil || len(list) != 1 || list[0].Turns != 0 {
		t.Fatalf("list %+v %v", list, err)
	}
	if _, err := LoadSession(dir, "missing"); !os.IsNotExist(err) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestLegacyJSONSessionsAreNotImported(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(CloseSessionStores)
	old := &Session{ID: "legacy-1", Title: "old chat", Cwd: "/x", UpdatedAt: time.Now().Add(-time.Hour), Turns: []Turn{
		{Role: llm.RoleUser, Text: "q"}, {Role: llm.RoleAssistant, Text: "a", CallID: "c", Code: "x"},
	}}
	if err := store.SaveJSON(filepath.Join(dir, "legacy-1.json"), old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := ListSessions(home)
	if err != nil || len(list) != 0 {
		t.Fatalf("legacy import: %v %v", list, err)
	}
}

func TestAttachmentsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/w", "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "see [Image #1]", Attachments: []Attachment{{N: 1, Mime: "image/png", SHA: "abc", W: 3, H: 4}}})
	s.Append(Turn{Role: llm.RoleUser, Kind: KindView, Text: "v"})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	turns := got.Snapshot()
	if turns[1].Kind != KindView || turns[0].Kind != "" {
		t.Fatalf("kind: %+v", turns)
	}
	if len(turns[0].Attachments) != 1 || turns[0].Attachments[0] != (Attachment{N: 1, Mime: "image/png", SHA: "abc", W: 3, H: 4}) || len(turns[1].Attachments) != 0 {
		t.Fatalf("%+v", turns)
	}
}
