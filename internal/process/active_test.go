package process

import (
	"context"
	"testing"
	"time"
)

func TestActiveIncludesCompletionCallback(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	s, _ := newSup(t, func(Task) { close(entered); <-release })
	task := start(t, s, "s1", "true", nil)
	<-entered
	// The DB status is already terminal, but the hook can still launch an agent
	// continuation. An updater must not interpret this interval as idle.
	if task, err := s.Get("s1", task.ID); err != nil || !task.Terminal() {
		t.Fatalf("task %v: %v", task, err)
	}
	if active, err := s.HasActive(); err != nil || !active {
		t.Fatalf("active %v: %v", active, err)
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := s.Wait(ctx, "s1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	s.wg.Wait()
	if active, err := s.HasActive(); err != nil || active {
		t.Fatalf("active %v: %v", active, err)
	}
}
