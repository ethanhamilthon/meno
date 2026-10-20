//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// askUser: the model asks, the user answers in the panel above the composer.
func askHandler(code string, final string) func(q *llmReq) {
	return func(q *llmReq) {
		if q.Mentions("[[answered]]") {
			q.Text(final)
			return
		}
		time.Sleep(300 * time.Millisecond) // time to type a draft
		q.Tool("", code)
	}
}

const askFormJSON = `{"questions":[{"id":"db","text":"Which database?","type":"single","options":[{"id":"sq","label":"sqlite"},{"id":"pg","label":"postgres"}],"recommendation":"postgres"},{"id":"port","text":"Which port?","type":"text"}]}`

func TestAskUserFormSubmit(t *testing.T) {
	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = askHandler("out=$(meno ask - <<'EOF'\n"+askFormJSON+"\nEOF\n); echo \"[[answered]] $out\"", "great, configured")
			s := start(t, options{fake: f, proto: proto})
			s.Say("set up storage")
			s.Type("my unfinished draft")
			scr := s.WaitForWithin("agent asks", 10*time.Second)
			for _, want := range []string{"Which database?", "sqlite", "postgres", "suggested: postgres", "Next", "Cancel", "question 1/2"} {
				if !strings.Contains(scr, want) {
					t.Fatalf("form misses %q:\n%s", want, scr)
				}
			}
			if strings.Contains(scr, "my unfinished draft") {
				t.Fatalf("draft should be set aside:\n%s", scr)
			}
			s.Send(Down)
			s.Send(Enter) // selects postgres and advances to question 2 (text, starts in typing mode)
			s.Type("8080")
			s.Send(Esc) // leaves typing; the form stays
			time.Sleep(500 * time.Millisecond)
			scr = s.Screen()
			if !strings.Contains(scr, "agent asks") {
				t.Fatalf("Esc must not close the form:\n%s", scr)
			}
			s.Send(Enter) // Esc after typing moved the focus to Next, which submits on the last question
			scr = s.WaitFor("great, configured")
			if !strings.Contains(scr, "pg") || !strings.Contains(scr, "8080") {
				t.Fatalf("\n%s", scr)
			}
			s.WaitFor("my unfinished draft") // the draft is back
		})
	}
}

func TestAskUserCancelFailsTheCommand(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler(`meno ask - <<'EOF'
{"questions":[{"id":"a","text":"first?","type":"text"}]}
EOF
echo "[[answered]] exit=$?"`, "noted")
	s := start(t, options{fake: f})
	s.Say("ask once")
	s.WaitFor("agent asks")
	s.Send(Esc) // leaves typing and focuses Submit
	time.Sleep(500 * time.Millisecond)
	s.Send("\t") // Cancel
	s.Send(Enter)
	scr := s.WaitFor("noted")
	if !strings.Contains(scr, "exit=1") {
		t.Fatalf("a cancelled form must fail the command:\n%s", scr)
	}
}

// 20. A skill edited between two messages is reported in the second request only,
// and the system prompt stays the same.
func TestSkillEditProducesSystemEvent(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("noted") }
	s := start(t, options{fake: f})
	dir := filepath.Join(s.work, ".meno", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dir, "style.md")
	if err := os.WriteFile(skill, []byte("---\ndescription: code style\n---\nv1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Say("first message")
	s.WaitFor("100/20")
	if err := os.WriteFile(skill, []byte("---\ndescription: code style\n---\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Say("second message")
	s.WaitFor("200/40")
	bodies := f.Bodies()
	if len(bodies) != 2 {
		t.Fatalf("requests: %d", len(bodies))
	}
	if strings.Contains(bodies[0], "skills-updated") {
		t.Fatal("the first request must not carry an event")
	}
	if !strings.Contains(bodies[1], "skills-updated") || !strings.Contains(bodies[1], "was updated: reread") || !strings.Contains(bodies[1], "style.md") {
		t.Fatalf("second request lacks the event:\n%s", bodies[1])
	}
	if strings.Contains(s.Screen(), "system-event") {
		t.Fatal("events must not show in the timeline")
	}
}
