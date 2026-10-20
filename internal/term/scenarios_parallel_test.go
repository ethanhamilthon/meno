//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

// Two calls wait for each other's marker file, so they finish only when they
// run at once. Both outputs and both results reach the model.
func TestParallelCallsRunTogether(t *testing.T) {
	for _, protocol := range allProtocols {
		t.Run(protocol, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) {
				if q.Mentions("out-alpha") && q.Mentions("out-beta") {
					q.Text("both calls finished")
					return
				}
				q.Tools("",
					"touch pa; while [ ! -e pb ]; do sleep 0.05; done; echo out-alpha",
					"touch pb; while [ ! -e pa ]; do sleep 0.05; done; echo out-beta")
			}
			s := start(t, options{fake: f, proto: protocol})
			s.Say("run both")
			scr := s.WaitFor("both calls finished")
			if !strings.Contains(scr, "out-alpha") || !strings.Contains(scr, "out-beta") {
				t.Fatalf("both outputs belong on screen:\n%s", scr)
			}
			if f.Requests() != 2 {
				t.Fatalf("requests: %d", f.Requests())
			}
		})
	}
}

// A message written while a call runs moves it to a task; the model answers at
// once, sees the task ID and can inspect the task. The process keeps running.
func TestHumanMessagePromotesRunningCall(t *testing.T) {
	for _, protocol := range allProtocols {
		t.Run(protocol, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) {
				switch {
				case q.Mentions("moved to background task t"):
					q.Text("the call is now a task")
				case q.N == 1:
					q.Tools("", "echo started-slow; sleep 60")
				default:
					q.Text("unexpected request")
				}
			}
			s := start(t, options{fake: f, proto: protocol})
			s.Say("start the slow thing")
			s.WaitFor("started-slow")
			s.Say("never mind, carry on")
			s.WaitForWithin("the call is now a task", 20*time.Second)
			s.Never("unexpected request", 500*time.Millisecond)
			bodies := f.Bodies()
			last := bodies[len(bodies)-1]
			if !strings.Contains(last, "meno tasks logs") || !strings.Contains(last, "never mind, carry on") {
				t.Fatalf("the request lacks the task reference or the message:\n%s", last)
			}
		})
	}
}

// Stop cancels the foreground call but not a task the call started.
func TestStopLeavesTasksAlive(t *testing.T) {
	for _, protocol := range allProtocols {
		t.Run(protocol, func(t *testing.T) {
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) {
				switch {
				case q.Mentions("tasklist-check"):
					q.Text("listed")
				case q.Mentions("list the tasks"):
					q.Tool("", "meno tasks list --json; echo tasklist-check")
				default:
					q.Tool("", "meno tasks start 'sleep 61' --name keepme; sleep 60")
				}
			}
			s := start(t, options{fake: f, proto: protocol})
			s.Say("make a task and wait")
			s.WaitFor("sleep 60")
			time.Sleep(time.Second) // the task exists once the command has run
			s.Send("\x03")
			s.WaitFor("INSERT")
			s.Say("list the tasks")
			scr := s.WaitFor("listed")
			if !strings.Contains(scr, "running") {
				t.Fatalf("the task did not survive Stop:\n%s", scr)
			}
		})
	}
}
