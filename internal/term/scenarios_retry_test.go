//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Provider errors: transient ones are retried with a visible countdown.
func TestRetryAfterTransientErrors(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		switch q.N {
		case 1:
			q.Fail(503, "1")
		case 2:
			q.Drop("partial that must vanish")
		default:
			q.Text("recovered fine")
		}
	}
	s := start(t, options{fake: f})
	s.Say("please")
	s.WaitFor("retry 2/5")
	s.WaitFor("503")
	s.WaitFor("retry 3/5")
	scr := s.WaitFor("recovered fine")
	if strings.Contains(scr, "partial that must vanish") {
		t.Fatalf("partial output of a dropped stream stayed:\n%s", scr)
	}
	if f.Requests() != 3 {
		t.Fatalf("requests: %d", f.Requests())
	}
}

// A bad key is not retried; the message stays in the timeline.
func TestAuthErrorIsNotRetried(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Fail(401, "") }
	s := start(t, options{fake: f})
	s.Say("anyone there")
	s.WaitFor("401")
	s.Never("retry 2/5", time.Second)
	if f.Requests() != 1 {
		t.Fatalf("requests: %d", f.Requests())
	}
	s.WaitFor("anyone there")
}

// Ctrl+C cancels the wait between retries.
func TestCtrlCDuringBackoff(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Fail(429, "30") }
	s := start(t, options{fake: f})
	s.Say("slow down")
	s.WaitFor("retry 2/5 in")
	s.Send(CtrlC)
	s.WaitFor("[aborted]")
	lines := strings.Split(s.Screen(), "\n")
	if status := lines[len(lines)-1]; strings.Contains(status, "retry") {
		t.Fatalf("status bar still shows a retry: %q", status)
	}
	time.Sleep(300 * time.Millisecond)
	if f.Requests() != 1 {
		t.Fatalf("requests: %d", f.Requests())
	}
}

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

const askFormJS = `askUser({questions: [
  {id: "db", text: "Which database?", type: "single", options: [{id: "sq", label: "sqlite"}, {id: "pg", label: "postgres"}], recommendation: "postgres"},
  {id: "port", text: "Which port?", type: "text"}
]})`

func TestAskUserFormSubmit(t *testing.T) {
	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = askHandler("const r = "+askFormJS+"; console.log('[[answered]] ' + r.answers.map(a => a.id + '=' + a.selected.join(',') + a.text).join(' '))", "great, configured")
			s := start(t, options{fake: f, proto: proto})
			s.Say("set up storage")
			s.Type("my unfinished draft")
			scr := s.WaitForWithin("agent asks", 10*time.Second)
			for _, want := range []string{"Which database?", "sqlite", "postgres", "suggested: postgres", "Submit", "Cancel", "question 1/2"} {
				if !strings.Contains(scr, want) {
					t.Fatalf("form misses %q:\n%s", want, scr)
				}
			}
			if strings.Contains(scr, "my unfinished draft") {
				t.Fatalf("draft should be set aside:\n%s", scr)
			}
			s.Send(Down)
			s.Send(Enter) // postgres
			s.Send("]")   // next question; text questions start in typing mode
			s.Type("8080")
			s.Send(Esc) // leaves typing; the form stays
			time.Sleep(500 * time.Millisecond)
			scr = s.Screen()
			if !strings.Contains(scr, "agent asks") {
				t.Fatalf("Esc must not close the form:\n%s", scr)
			}
			s.Send(Enter) // Esc after typing moved the focus to Submit
			scr = s.WaitFor("great, configured")
			if !strings.Contains(scr, "db=pgport=8080") && !strings.Contains(scr, "db=pg") {
				t.Fatalf("\n%s", scr)
			}
			s.WaitFor("my unfinished draft") // the draft is back
		})
	}
}

func TestAskUserCancelThrowsAndTimeoutIsTyped(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler(`
		let r = [];
		try { askUser({questions: [{id: "a", text: "first?", type: "text"}]}) } catch (e) { r.push(e.name) }
		try { askUser({timeout: 1, questions: [{id: "b", text: "second?", type: "text"}]}) } catch (e) { r.push(e.name) }
		console.log('[[answered]] ' + r.join(' | '))`, "noted")
	s := start(t, options{fake: f})
	s.Say("ask twice")
	s.WaitFor("agent asks")
	s.Send(Esc) // leaves typing and focuses Submit
	time.Sleep(500 * time.Millisecond)
	s.Send("\t") // Cancel
	s.Send(Enter)
	scr := s.WaitFor("noted")
	if !strings.Contains(scr, "AskCancelled | AskTimeout") {
		t.Fatalf("\n%s", scr)
	}
}

func TestCtrlCDuringQuestion(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler(`askUser({questions: [{id: "a", text: "waiting forever?", type: "text"}]})`, "never")
	s := start(t, options{fake: f})
	s.Say("ask")
	s.WaitFor("waiting forever?")
	s.Send(CtrlC)
	s.WaitFor("[aborted]")
	s.WaitGone("agent asks")
	time.Sleep(300 * time.Millisecond)
	if f.Requests() != 1 {
		t.Fatalf("requests: %d", f.Requests())
	}
	// The composer works normally again.
	s.Type("hello again")
	s.WaitFor("hello again")
}

func TestAskFormOnNarrowTerminal(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler(`const r = askUser({questions: [{id: "a", text: "A rather long question that has to wrap on a narrow terminal, word by word", type: "single", options: [{id: "1", label: "one"}, {id: "2", label: "two"}]}]}); console.log('[[answered]] ' + r.answers[0].selected[0])`, "all done")
	s := start(t, options{fake: f, cols: 40, rows: 24})
	s.Say("go")
	s.WaitFor("agent asks")
	s.WaitFor("Submit")
	for i, l := range strings.Split(s.Screen(), "\n") {
		if n := len([]rune(l)); n > 40 {
			t.Fatalf("row %d is %d wide: %q", i, n, l)
		}
	}
	if scr := s.Screen(); !strings.Contains(scr, "narrow terminal") || !strings.Contains(scr, "Cancel") {
		t.Fatalf("question or actions cut off:\n%s", scr)
	}
	s.Resize(100, 32)
	s.WaitFor("[ Submit ]")
	s.Send("1")
	s.Send("\t\t\t")
	s.Send("\t")
	s.Send(Enter)
	s.WaitFor("all done")
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
