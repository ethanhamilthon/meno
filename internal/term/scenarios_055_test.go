//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryYankReturnsToInput(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("history-copy-marker") }
	s := start(t, options{fake: f, cols: 100, rows: 24})
	s.Say("first message")
	s.WaitFor("history-copy-marker")
	s.WaitFor("Type your next task here")
	s.Cmd("edit")
	s.Send("ggvjy")
	s.Type("draft-after-copy")
	screen := s.WaitFor("draft-after-copy")
	rows := strings.Split(strings.TrimSuffix(screen, "\n"), "\n")
	if !strings.Contains(rows[len(rows)-3], "draft-after-copy") {
		t.Fatalf("typing did not reach composer:\n%s", screen)
	}
	if !strings.Contains(screen, "history-copy-marker") {
		t.Fatal("history changed")
	}
}

func TestNamespacedSkillSelection(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("skill-send-complete") }
	s := start(t, options{fake: f, cols: 100, rows: 24})
	dir := filepath.Join(s.work, ".meno", "skills")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte("---\ndescription: review marker\n---\nReview carefully.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s.Cmd("skills")
	s.WaitFor("review marker")
	s.Send(Esc)
	s.WaitGone("search skills")
	s.Type("/skill:rev")
	s.WaitFor("review marker")
	s.Send("\t")
	s.WaitFor("/skill:review")
	s.Send(Enter)
	s.WaitFor("skill-send-complete")
}
