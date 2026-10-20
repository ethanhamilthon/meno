//go:build termtest

package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model picked with :model applies to this session; the next request carries
// the new model and a model-changed event.
func TestModelSwitchProducesSystemEvent(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("noted") }
	s := start(t, options{fake: f})
	s.Say("first message")
	s.WaitFor("↑100 ↓20")
	s.Cmd("model")
	s.WaitFor("other-model")
	s.Send(Down + Enter)
	s.WaitGone("search models")
	s.WaitFor("other-model |")
	s.Say("second message")
	waitRequests(t, f, 2)
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
	s.WaitFor("↑100 ↓20")
	s.Cmd("cd sub")
	s.WaitFor("working directory")
	s.Say("second")
	waitRequests(t, f, 2)
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
