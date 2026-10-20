//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
)

var protocols = []string{"chat", "responses"}

// 1. First run: no provider, so the login list opens and cannot be closed. Add
// an API-key provider there, pick a model in :model, talk.
func TestFirstRunSetupThroughDialogs(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("first ", "answer") }
	s := start(t, options{firstRun: true})
	s.WaitFor("add a provider to start")
	s.Send(Esc)
	time.Sleep(150 * time.Millisecond)
	s.WaitFor("add a provider to start") // esc must not close the list
	s.Send("j" + Enter)                  // API key · Chat Completions
	s.WaitFor("base url")
	s.Send(Enter) // edit name
	s.Type("fake")
	s.Send(Enter)
	s.Send("j" + Enter)
	s.Type(f.URL())
	s.Send(Enter)
	s.Send("j" + Enter)
	s.Type("secret-key")
	s.Send(Enter)
	s.WaitFor("ready")
	s.Send(Esc)
	time.Sleep(150 * time.Millisecond)
	s.Send(Esc)
	s.WaitGone("LOGIN")

	s.Cmd("model")
	s.WaitFor("fake-model")
	s.WaitFor("other-model")
	s.Send(Enter)
	s.WaitGone("search models")
	s.WaitFor("fake-model (")

	s.Say("hello again")
	s.WaitFor("first answer")
	if f.Requests() != 1 {
		t.Fatalf("requests: %d", f.Requests())
	}
}

// 2. A message streams in and the turn completes, in both protocols.
func TestStreamingAnswer(t *testing.T) {
	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) { q.Text("Hel", "lo ", "from ", proto) }
			s := start(t, options{fake: f, proto: proto})
			s.WaitFor("give a task")
			s.Say("greet me")
			s.WaitFor("Hello from " + proto)
			s.WaitFor("100/20") // usage arrives when the stream completes
			if strings.Contains(s.Screen(), "retry") {
				t.Fatal("unexpected retry")
			}
		})
	}
}

// 3. The model runs js: code and output show up and the result is fed back.
func TestJSToolTurn(t *testing.T) {
	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) {
				if q.Mentions("1764") {
					q.Text("the answer is 1764")
					return
				}
				q.Tool("let me compute", "echo result=$((42 * 42))")
			}
			s := start(t, options{fake: f, proto: proto})
			s.Say("what is 42 squared")
			scr := s.WaitFor("the answer is 1764")
			for _, want := range []string{"42 * 42", "result=1764"} {
				if !strings.Contains(scr, want) {
					t.Fatalf("timeline misses %q:\n%s", want, scr)
				}
			}
			if f.Requests() != 2 {
				t.Fatalf("requests: %d", f.Requests())
			}
		})
	}
}

// 4. Ctrl+C in the middle of a stream aborts the turn; the next one works.
func TestCtrlCDuringStream(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.N == 1 {
			q.Hang("half of an answer")
			return
		}
		q.Text("second turn fine")
	}
	s := start(t, options{fake: f})
	s.Say("go")
	s.WaitFor("half of an answer")
	s.Send(CtrlC)
	s.WaitFor("[aborted]")
	if s.Exited() {
		t.Fatal("ctrl+c must abort the turn, not quit")
	}
	s.Say("again")
	s.WaitFor("second turn fine")
}

// 5. :new starts an empty session, :sessions brings the old one back.
func TestNewAndResume(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("remember-me-answer") }
	s := start(t, options{fake: f})
	s.Say("my first question")
	s.WaitFor("remember-me-answer")
	s.WaitFor("100/20")
	s.Cmd("new")
	s.WaitGone("remember-me-answer")
	s.Cmd("sessions")
	s.WaitFor("my first question")
	s.Type("first quest") // the search box has the focus
	s.Send(Enter)
	s.WaitFor("remember-me-answer")
}

// 5b. A session survives a restart of the program.
func TestResumeAfterRestart(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("persisted-answer") }
	home := t.TempDir()
	s := start(t, options{fake: f, home: home})
	s.Say("question before restart")
	s.WaitFor("persisted-answer")
	s.WaitFor("100/20")
	s.Cmd("q")
	if err := s.WaitExit(); err != nil {
		t.Fatal(err)
	}
	s2 := start(t, options{home: home})
	s2.WaitFor("give a task")
	s2.Cmd("sessions")
	s2.WaitFor("question before")
	s2.Type("before restart")
	s2.Send(Enter)
	s2.WaitFor("persisted-answer")
}

// 6. Resizing the window re-lays the screen out, down to a very narrow terminal.
func TestResizeAndNarrowTerminal(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		q.Text("A fairly long answer that has to be wrapped differently when the window changes its width.")
	}
	s := start(t, options{fake: f})
	s.Say("wrap please")
	s.WaitFor("when the window")
	s.Resize(60, 20)
	s.WaitFor("INSERT")
	lines := strings.Split(s.Screen(), "\n")
	if len(lines) != 20 || !strings.Contains(lines[19], "INSERT") {
		t.Fatalf("status bar not on the last row of 60x20:\n%s", s.Screen())
	}
	s.Resize(40, 14)
	s.WaitFor("fake-model")
	for i, l := range strings.Split(s.Screen(), "\n") {
		if n := len([]rune(l)); n > 40 {
			t.Fatalf("row %d is %d wide in a 40 column terminal: %q", i, n, l)
		}
	}
	s.Resize(100, 32)
	s.WaitFor("INSERT")
	s.Say("still alive")
	s.WaitFor("still alive")
	if s.Exited() {
		t.Fatal("meno died")
	}
}

// 7. Cyrillic, emoji and Markdown are drawn, not mangled.
func TestUnicodeAndMarkdown(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		q.Text("Привет, **жирный** мир 🙂\n\n", "- первый пункт\n- второй пункт\n\n", "```go\nfmt.Println(\"привет\")\n```\n")
	}
	s := start(t, options{fake: f})
	s.Say("скажи привет")
	scr := s.WaitFor("fmt.Println")
	for _, want := range []string{"Привет, жирный мир", "первый пункт", "второй пункт", "привет"} {
		if !strings.Contains(scr, want) {
			t.Fatalf("misses %q:\n%s", want, scr)
		}
	}
	for _, bad := range []string{"**", "```"} {
		if strings.Contains(scr, bad) {
			t.Fatalf("raw markdown %q on screen:\n%s", bad, scr)
		}
	}
	// The user's own cyrillic message is in the timeline too.
	if !strings.Contains(scr, "скажи привет") {
		t.Fatalf("user message missing:\n%s", scr)
	}
}

// 8. :q ends the process cleanly and gives the terminal back.
func TestQuitRestoresTerminal(t *testing.T) {
	f := newFakeLLM(t)
	s := start(t, options{fake: f})
	s.WaitFor("give a task")
	if s.vt.Mode()&vt10x.ModeAltScreen == 0 {
		t.Fatal("meno should use the alternate screen")
	}
	s.Cmd("q")
	if err := s.WaitExit(); err != nil {
		t.Fatalf("exit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if s.vt.Mode()&vt10x.ModeAltScreen != 0 {
		t.Fatal("alternate screen not left")
	}
	if !s.vt.CursorVisible() {
		t.Fatal("cursor left hidden")
	}
}
