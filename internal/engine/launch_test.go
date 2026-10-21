package engine

import (
	"context"
	"testing"
)

func TestClosedEngineDoesNotLaunchContinuation(t *testing.T) {
	f := &fakeProvider{seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	e.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.mu.Lock()
	s.running = true
	s.cancel = cancel
	s.mu.Unlock()
	e.launch(ctx, s, "", true)
	e.runs.Wait()
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	if calls != 0 {
		t.Fatalf("closed engine launched %d provider calls", calls)
	}
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if running {
		t.Fatal("rejected launch left session running")
	}
	if ctx.Err() == nil {
		t.Fatal("rejected launch context was not cancelled")
	}
}
