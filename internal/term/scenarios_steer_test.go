//go:build termtest

package term

import (
	"strings"
	"testing"
)

func TestConsumedSteeringLeavesQueueWhileRunContinues(t *testing.T) {
	for _, protocol := range protocols {
		t.Run(protocol, func(t *testing.T) {
			first := make(chan struct{})
			second := make(chan struct{})
			f := newFakeLLM(t)
			f.Handler = func(q *llmReq) {
				q.head()
				if q.N == 1 {
					q.TextPartial("working on the first step")
					select {
					case <-first:
					case <-q.r.Context().Done():
						return
					}
				} else {
					q.TextPartial("steering reached the model")
					select {
					case <-second:
					case <-q.r.Context().Done():
						return
					}
				}
				q.finish()
			}
			s := start(t, options{fake: f, proto: protocol})
			s.Say("start working")
			s.WaitFor("working on the first step")
			s.Say("follow this steering note")
			s.WaitFor("↳ queued")
			close(first)
			screen := s.WaitFor("steering reached the model")
			s.WaitGone("↳ queued")
			bodies := f.Bodies()
			if len(bodies) < 2 || !strings.Contains(bodies[1], "follow this steering note") {
				t.Fatalf("steering missing from next request; screen:\n%s", screen)
			}
			close(second)
		})
	}
}
