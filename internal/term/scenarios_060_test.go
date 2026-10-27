//go:build termtest

package term

import (
	"testing"
	"time"
)

// The first-run screen promises that ctrl+c quits.
func TestFirstRunCtrlCQuits(t *testing.T) {
	s := start(t, options{firstRun: true})
	s.Send(CtrlC)
	done := make(chan error, 1)
	go func() { done <- s.WaitExit() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("ctrl+c did not quit:\n%s", s.Screen())
	}
}
