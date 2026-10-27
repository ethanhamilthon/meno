//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opening meno and starting new sessions leaves nothing in /sessions; the
// first message is what creates the session, and it is named at once.
func TestSessionExistsOnlyAfterTheFirstMessage(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("answer-marker") }
	home := t.TempDir()
	a := start(t, options{fake: f, home: home, cols: 110, rows: 28})
	a.Cmd("new")
	a.Cmd("new")
	a.Cmd("resume")
	screen := a.WaitFor("no matching sessions")
	if strings.Contains(screen, "new session") {
		t.Fatalf("an empty session is listed:\n%s", screen)
	}
	a.Send(Esc)
	a.WaitGone("no matching sessions")
	a.Say("name this session please")
	a.WaitFor("answer-marker")
	a.Cmd("resume")
	a.WaitFor("tab filter")
	a.Send(Esc)
	a.WaitGone("tab filter")
	_ = a.cmd.Process.Kill()
	<-a.exited
	b := start(t, options{fake: f, home: home, cols: 110, rows: 28})
	b.Cmd("resume")
	b.WaitFor("name this session please")
}

// /plann finds the skill "planning" and inserts it without running or sending.
func TestSlashFindsSkillWithoutTheSkillPrefix(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("must-not-be-asked") }
	s := start(t, options{fake: f, cols: 110, rows: 28})
	dir := filepath.Join(s.work, ".meno", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "planning.md"), []byte("---\ndescription: plan marker\n---\nWrite a plan.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Cmd("skills")
	s.WaitFor("plan marker")
	s.Send(Esc)
	s.WaitGone("tab auto-load")
	s.Type("/plann")
	s.WaitFor("/skill:planning")
	s.Send(Enter)
	screen := s.WaitFor("planning")
	if strings.Contains(screen, "must-not-be-asked") {
		t.Fatalf("selecting a skill sent the draft:\n%s", screen)
	}
}
