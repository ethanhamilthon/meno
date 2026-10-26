//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"testing"
)

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
	s.WaitGone("tab auto-load")
	s.Type("/skill:rev")
	s.WaitFor("review marker")
	s.Send("\t")
	s.WaitFor("/skill:review")
	s.Send(Enter)
	s.WaitFor("skill-send-complete")
}
