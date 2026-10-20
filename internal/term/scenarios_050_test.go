//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

func TestCompletionWaitsUntilResultIsVisible(t *testing.T) {
	f := newFakeLLM(t)
	release := make(chan struct{})
	defer close(release)
	f.Handler = func(q *llmReq) {
		select {
		case <-release:
		case <-time.After(700 * time.Millisecond):
		}
		q.Text("completed-background-result")
	}
	s := start(t, options{fake: f})
	s.Say("finish while help covers result")
	s.Cmd("help")
	s.WaitFor("keybindings")
	s.WaitFor("1 done")
	s.Send(Esc)
	s.WaitFor("completed-background-result")
	s.WaitFor("0 done")
	s.Cmd("context")
	s.WaitFor("Session total")
	s.Send(Esc)
	if !strings.Contains(s.Screen(), "COMMAND") {
		s.WaitFor("COMMAND")
	}
}

func TestBashHeadersUseTitlesAndFinalRunStatistics(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.Mentions("bash-output-marker") {
			q.Text("bash-display-complete")
		} else {
			q.Tool("", "printf '%s\\n' bash-output-marker")
		}
	}
	s := start(t, options{fake: f})
	s.Say("inspect Bash header")
	s.WaitFor("bash-display-complete")
	s.WaitFor("1 bash")
	if !strings.Contains(s.Screen(), "Inspect shell output") {
		t.Fatal(s.Screen())
	}
}
