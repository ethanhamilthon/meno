//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
)

var protocols = []string{"chat", "responses"}

// 1. First run: no provider, add one in :providers, pick a model in :model, talk.
func TestFirstRunSetupThroughDialogs(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("first ", "answer") }
	s := start(t, options{})
	s.WaitFor("give a task")
	s.Say("hello")
	s.WaitFor("provider not configured")

	s.Cmd("providers")
	s.WaitFor("new provider")
	s.Send("n")
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
	s.WaitFor("fake")
	s.Send(CtrlS)
	s.WaitFor("provider saved")
	s.Send(Esc)
	time.Sleep(150 * time.Millisecond)
	s.Send(Esc)
	time.Sleep(150 * time.Millisecond)

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
			s.WaitFor("↓ 100") // usage arrives when the stream completes
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
				q.Tool("let me compute", "const n = 42 * 42; console.log('result=' + n); n")
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

// 5. :new starts an empty session, :resume brings the old one back.
func TestNewAndResume(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("remember-me-answer") }
	s := start(t, options{fake: f})
	s.Say("my first question")
	s.WaitFor("remember-me-answer")
	s.WaitFor("↓ 100")
	s.Cmd("new")
	s.WaitGone("remember-me-answer")
	s.Cmd("resume")
	s.WaitFor("my first question")
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
	s.WaitFor("↓ 100")
	s.Cmd("q")
	if err := s.WaitExit(); err != nil {
		t.Fatal(err)
	}
	s2 := start(t, options{home: home})
	s2.WaitFor("give a task")
	s2.Cmd("resume")
	s2.WaitFor("question before restart")
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

func TestAskUserChoice(t *testing.T) {
	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = askHandler("const a = askUser('Which database?', {choices: ['sqlite', 'postgres']}); console.log('[[answered]] picked=' + a)", "great, postgres it is")
			s := start(t, options{fake: f, proto: proto})
			s.Say("set up storage")
			s.Type("my unfinished draft")
			scr := s.WaitForWithin("agent asks", 10*time.Second)
			t.Logf("requests so far: %d", f.Requests())
			for _, want := range []string{"Which database?", "1 sqlite", "2 postgres", "Your own answer", "waiting for your answer"} {
				if !strings.Contains(scr, want) {
					t.Fatalf("panel misses %q:\n%s", want, scr)
				}
			}
			if strings.Contains(scr, "my unfinished draft") {
				t.Fatalf("draft should be set aside:\n%s", scr)
			}
			s.Send("2")
			scr = s.WaitFor("great, postgres it is")
			if !strings.Contains(scr, "picked=postgres") || strings.Contains(scr, "agent asks") {
				t.Fatalf("\n%s", scr)
			}
			s.WaitFor("my unfinished draft") // the draft is back
		})
	}
}

func TestAskUserFreeTextAndOwnAnswer(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler("const p = askUser('Which port?'); const c = askUser('Pick one', {choices: ['x', 'y']}); console.log('[[answered]] port=' + p + ' color=' + c)", "configured")
	s := start(t, options{fake: f})
	s.Say("configure")
	s.WaitFor("type your answer below") // the panel's hint; the code in the timeline also names the question
	s.Type("8080")
	s.Send(Enter)
	s.WaitFor("1-9 choose")
	s.Send("3") // Your own answer
	s.WaitFor("type your answer ·")
	s.Type("turquoise")
	s.Send(Enter)
	scr := s.WaitFor("configured")
	if !strings.Contains(scr, "port=8080 color=turquoise") {
		t.Fatalf("\n%s", scr)
	}
}

func TestAskUserDeclineAndTimeoutAreErrors(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler(`
		let r = [];
		try { askUser('first?') } catch (e) { r.push(e.message) }
		try { askUser('second?', {timeout: 1}) } catch (e) { r.push(e.message) }
		console.log('[[answered]] ' + r.join(' | '))`, "noted")
	s := start(t, options{fake: f})
	s.Say("ask twice")
	s.WaitFor("ctrl+x decline")
	s.Send("\x18") // Ctrl+X declines
	s.WaitFor("answer within")
	scr := s.WaitFor("noted")
	if !strings.Contains(scr, "user declined to answer | ") || !strings.Contains(scr, "user did not answer within 1s") {
		t.Fatalf("\n%s", scr)
	}
}

func TestCtrlCDuringQuestion(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler("askUser('waiting forever?')", "never")
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

func TestAskPanelOnNarrowTerminal(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = askHandler("const a = askUser('A rather long question that has to wrap on a narrow terminal, word by word', {choices: ['one', 'two']}); console.log('[[answered]] ' + a)", "all done")
	s := start(t, options{fake: f, cols: 40, rows: 24})
	s.Say("go")
	s.WaitFor("agent asks")
	s.WaitFor("enter esc")
	for i, l := range strings.Split(s.Screen(), "\n") {
		if n := len([]rune(l)); n > 40 {
			t.Fatalf("row %d is %d wide: %q", i, n, l)
		}
	}
	if scr := s.Screen(); !strings.Contains(scr, "word by word") {
		t.Fatalf("question cut off:\n%s", scr)
	}
	s.Resize(100, 32)
	s.WaitFor("enter confirm")
	s.Send("1")
	s.WaitFor("all done")
}
