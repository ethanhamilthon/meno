package agent

import (
	"strings"
	"testing"
	"time"

	"meno/internal/llm"
)

func seedSession(t *testing.T, home, cwd string, texts ...string) *Session {
	t.Helper()
	s := NewSession(cwd, "m")
	for i, tx := range texts {
		s.Append(Turn{Role: llm.RoleUser, Text: tx, Time: time.Now().Add(time.Duration(i) * time.Millisecond)})
	}
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestListSearchReadAcrossWorkdirsWithPagination(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	var ids []string
	for i := 0; i < 5; i++ {
		cwd := "/proj/a"
		if i%2 == 1 {
			cwd = "/proj/b"
		}
		ids = append(ids, seedSession(t, home, cwd, "hello 100%_ sure", "second").ID)
		time.Sleep(2 * time.Millisecond)
	}
	child := NewSession("/proj/a", "m")
	child.Kind, child.ParentID = KindChild, ids[0]
	child.Append(Turn{Role: llm.RoleUser, Text: "hello child secret"})
	if err := SaveSession(home, child); err != nil {
		t.Fatal(err)
	}
	var got []string
	cursor := ""
	for {
		p, err := ListSessionsPage(home, Query{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range p.Items {
			got = append(got, it.ID)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	if len(got) != 5 || got[0] != ids[4] || got[4] != ids[0] {
		t.Fatalf("list pages: %v want newest first of %v", got, ids)
	}
	if p, _ := ListSessionsPage(home, Query{Workdir: "/proj/b"}); len(p.Items) != 2 {
		t.Fatalf("workdir filter: %+v", p)
	}
	// Literal search: % and _ are not wildcards; children are excluded.
	seen := map[string]bool{}
	cursor = ""
	for pages := 0; pages < 10; pages++ {
		p, err := SearchSessions(home, "100%_", Query{Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range p.Items {
			key := it.ID + "/" + string(rune('0'+it.MessageID))
			if seen[key] || !strings.Contains(it.Excerpt, "100%_") {
				t.Fatalf("duplicate or bad hit %s: %+v", key, it)
			}
			seen[key] = true
		}
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	// Each session matches by its title (message 0) and by its first message.
	if len(seen) != 10 {
		t.Fatalf("search found %d hits over all pages, want 10", len(seen))
	}
	if p, _ := SearchSessions(home, "child secret", Query{}); len(p.Items) != 0 {
		t.Fatal("children must not be searchable")
	}
	if p, _ := SearchSessions(home, "1%0", Query{}); len(p.Items) != 0 {
		t.Fatal("% must be literal")
	}
	if _, err := SearchSessions(home, "  ", Query{}); err == nil {
		t.Fatal("empty query must be rejected")
	}
	// Reading: bounded, with a continuation, by stable message IDs; works on a
	// known child too (parent access is decided by the caller).
	r, err := ReadSessionHistory(home, ids[0], ReadOptions{Limit: 1})
	if err != nil || len(r.Records) != 1 || r.Next != 2 {
		t.Fatalf("read: %+v %v", r, err)
	}
	r, _ = ReadSessionHistory(home, ids[0], ReadOptions{From: r.Next})
	if len(r.Records) != 1 || r.Records[0].Text != "second" || r.Next != 0 {
		t.Fatalf("read next: %+v", r)
	}
}

func TestReadOmitsToolOutputAndThinkingUnlessAsked(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/w", "m")
	s.Append(Turn{Role: llm.RoleAssistant, Text: "x", Output: "big output", Thinking: "hmm", Code: "1"})
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	r, _ := ReadSessionHistory(home, s.ID, ReadOptions{})
	rec := r.Records[0]
	if rec.Output != "" || rec.Thinking != "" || rec.Omitted["output"] != 10 || rec.Omitted["thinking"] != 3 {
		t.Fatalf("%+v", rec)
	}
	r, _ = ReadSessionHistory(home, s.ID, ReadOptions{IncludeToolOutput: true, IncludeThinking: true})
	if r.Records[0].Output != "big output" || r.Records[0].Thinking != "hmm" || r.Records[0].Omitted != nil {
		t.Fatalf("%+v", r.Records[0])
	}
	if _, err := ReadSessionHistory(home, "nope", ReadOptions{}); err == nil {
		t.Fatal("unknown session")
	}
}
