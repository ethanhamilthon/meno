//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

// Say types a message and submits it.
func (s *session) Say(text string) {
	s.t.Helper()
	if strings.Contains(s.Screen(), "COMMAND") {
		s.Send(Esc)
		time.Sleep(150 * time.Millisecond)
	}
	s.Type(text)
	s.Send(Enter)
}

func waitRequests(t *testing.T, f *fakeLLM, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if f.Requests() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("requests: got %d, want %d", f.Requests(), n)
}
