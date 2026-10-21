package agent

import (
	"context"
	"strings"
	"testing"

	"meno/internal/llm"
)

// Fixtures here are invented and anonymous. Each test names the facts the model
// must still be able to read after a context operation, so a change in how the
// context is built shows up as a lost fact, not as a different token count.

func msgsText(msgs []llm.Msg) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Text + "\n")
	}
	return b.String()
}

func chat(s *Session, user, answer string) {
	s.Append(Turn{Role: llm.RoleUser, Text: user})
	s.Append(Turn{Role: llm.RoleAssistant, Text: answer})
}

// A decision made before a compaction survives it through the summary, and a
// decision made after it is read directly; the cut never keeps half of a tool
// exchange.
func TestDecisionsSurviveCompaction(t *testing.T) {
	s := NewSession(".", "m")
	chat(s, "Which database do we use?", "Decision: use SQLite for storage.")
	chat(s, "Which port?", "Decision: serve on port 8123.")
	s.Append(Turn{Role: llm.RoleAssistant, Text: "run", CallID: "c1", Code: "ls", Output: "a.go b.go", ReplayGroup: "g1"})
	chat(s, "Anything else?", "No.")
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "SUMMARY: storage is SQLite; port 8123."}}}}
	if _, _, err := Compact(context.Background(), cl, llm.Provider{}, "m", s); err != nil {
		t.Fatal(err)
	}
	chat(s, "Which theme?", "Decision: dark theme.")
	text := msgsText(s.ToMsgs())
	for _, fact := range []string{"SQLite", "8123", "dark theme", "No."} {
		if !strings.Contains(text, fact) {
			t.Errorf("lost fact %q:\n%s", fact, text)
		}
	}
	// A tool result never appears without the assistant message that asked for it.
	calls := map[string]bool{}
	for _, m := range s.ToMsgs() {
		if m.Role == llm.RoleAssistant && m.CallID != "" {
			calls[m.CallID] = true
		}
		if m.Role == llm.RoleTool && !calls[m.CallID] {
			t.Errorf("tool result %q without its call", m.CallID)
		}
	}
}

// The complete history is kept in storage after compaction, so an old fact can
// still be found and forked from even though it left the active context.
func TestCompactionKeepsFullHistoryInStorage(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(dir, "m")
	for i := 0; i < 6; i++ {
		chat(s, "question "+string(rune('A'+i)), "answer "+string(rune('A'+i)))
	}
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "SUMMARY"}}}}
	if _, _, err := Compact(context.Background(), cl, llm.Provider{}, "m", s); err != nil {
		t.Fatal(err)
	}
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	all, err := AllTurns(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 12 {
		t.Fatalf("history has %d turns, want all 12", len(all))
	}
	if len(s.Snapshot()) >= 12 {
		t.Fatal("compaction did not shorten the active context")
	}
}

// A fork stops at its boundary: it holds what was said up to it and nothing
// said later, and the later facts are not in what the model reads.
func TestForkContextStopsAtTheBoundary(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(dir, "m")
	chat(s, "first topic", "answer about the first topic")
	chat(s, "second topic", "answer about the second topic")
	chat(s, "third topic", "answer about the third topic")
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	all := s.Snapshot()
	f, _, err := ForkSession(dir, s.ID, all[3].HID, dir)
	if err != nil {
		t.Fatal(err)
	}
	text := msgsText(f.ToMsgs())
	for _, want := range []string{"first topic", "second topic"} {
		if !strings.Contains(text, want) {
			t.Errorf("fork lost %q", want)
		}
	}
	if strings.Contains(text, "third topic") {
		t.Error("fork contains a message after its boundary")
	}
}

// Images keep their place: the newest ones stay readable and the request never
// carries more than MaxRequestImages of them.
func TestOldImagesAreDroppedNotNewOnes(t *testing.T) {
	s := NewSession(".", "m")
	for i := 0; i < MaxRequestImages+3; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "look", Attachments: []Attachment{{N: i + 1, Mime: "image/png", SHA: strings.Repeat("a", 8)}}})
		s.Append(Turn{Role: llm.RoleAssistant, Text: "seen"})
	}
	n, last := 0, -1
	for _, m := range s.ToMsgs() {
		for _, im := range m.Images {
			if im.Path != "" {
				n++
				last = im.N
			}
		}
	}
	if n > MaxRequestImages {
		t.Fatalf("%d images in the request, limit %d", n, MaxRequestImages)
	}
	if last != MaxRequestImages+3 {
		t.Fatalf("newest image is %d", last)
	}
}
