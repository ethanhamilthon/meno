//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A model picked with :model applies to this session; the next request carries
// the new model and a model-changed event.
func TestModelSwitchProducesSystemEvent(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("noted") }
	s := start(t, options{fake: f})
	s.Say("first message")
	s.WaitFor("100/20")
	s.Cmd("model")
	s.WaitFor("other-model")
	s.Send(Down + Enter)
	s.WaitGone("search models")
	s.WaitFor("other-model (")
	s.Say("second message")
	s.WaitFor("200/40")
	bodies := f.Bodies()
	if len(bodies) != 2 {
		t.Fatalf("requests: %d", len(bodies))
	}
	if strings.Contains(bodies[0], "model-changed") || !strings.Contains(bodies[0], "fake-model") {
		t.Fatalf("first request:\n%s", bodies[0])
	}
	if !strings.Contains(bodies[1], "model-changed") || !strings.Contains(bodies[1], "other-model") {
		t.Fatalf("second request lacks the event:\n%s", bodies[1])
	}
}

// 21. Two sessions run at once: the second answers while the first is still
// waiting for the provider; the status bar and :sessions show it, and the slow
// one can be opened and finishes.
func TestParallelSessions(t *testing.T) {
	f := newFakeLLM(t)
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free) // never leave the handler hanging when the test fails
	f.Handler = func(q *llmReq) {
		if q.Mentions("slow-one") {
			<-release
			q.Text("answer-one")
			return
		}
		q.Text("answer-two")
	}
	s := start(t, options{fake: f})
	s.Say("slow-one")
	s.WaitFor("thinking") // the request is out
	s.Cmd("new")
	s.Say("quick-two")
	s.WaitFor("answer-two")
	s.WaitFor("| 1x◆")
	s.Cmd("sessions")
	s.WaitFor("slow-one")
	s.WaitFor("quick-two")
	s.Type("slow-one")
	s.Send(Enter)
	s.WaitGone("answer-two")
	s.WaitFor("slow-one")
	free()
	s.WaitFor("answer-one")
	if strings.Contains(s.Screen(), "answer-two") {
		t.Fatal("sessions must not mix")
	}
}

// 22. :q refuses while a turn runs and names the session; :q! stops everything.
func TestQuitRefusedWhileRunning(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Hang("working on it") }
	s := start(t, options{fake: f})
	s.Say("long job")
	s.WaitFor("working on it")
	s.Cmd("q")
	s.WaitFor("not quitting")
	s.WaitFor(":q!")
	if s.Exited() {
		t.Fatal(":q must not quit")
	}
	s.Cmd("q!")
	if err := s.WaitExit(); err != nil {
		t.Fatal(err)
	}
}

// 23. :cd changes the directory of the session; the model hears about it with
// the next message.
func TestCdProducesSystemEvent(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("fine") }
	s := start(t, options{fake: f})
	if err := os.Mkdir(filepath.Join(s.work, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.Say("first")
	s.WaitFor("100/20")
	s.Cmd("cd sub")
	s.WaitFor("working directory")
	s.Say("second")
	s.WaitFor("200/40")
	bodies := f.Bodies()
	if strings.Contains(bodies[0], "cwd-changed") {
		t.Fatal("no event before the change")
	}
	if !strings.Contains(bodies[1], "cwd-changed") || !strings.Contains(bodies[1], "sub") {
		t.Fatalf("second request lacks the event:\n%s", bodies[1])
	}
	s.Cmd("cd nowhere")
	s.WaitFor("no such directory")
}

// 24. :reset is gone: the JS runtime is fresh on every call, so there is
// nothing to reset.
func TestResetCommandIsGone(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("ok then") }
	s := start(t, options{fake: f})
	s.Cmd("reset")
	s.WaitFor("unknown command :reset")
}
