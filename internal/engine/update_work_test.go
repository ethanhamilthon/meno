package engine

import (
	"context"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/process"
)

func TestUpdateWorkCountsRunsQueuesAndBackgroundTasks(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	assertBusy := func(want bool) {
		t.Helper()
		busy, err := e.HasUpdateWork(context.Background())
		if err != nil || busy != want {
			t.Fatalf("busy %v want %v: %v", busy, want, err)
		}
	}
	assertBusy(false)
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	assertBusy(true)
	s.mu.Lock()
	s.running = false
	s.compacting = true
	s.mu.Unlock()
	assertBusy(true)
	s.mu.Lock()
	s.compacting = false
	s.mu.Unlock()
	assertBusy(false)

	e.mu.Lock()
	e.activeRuns++
	e.mu.Unlock()
	assertBusy(true)
	e.mu.Lock()
	e.activeRuns--
	e.mu.Unlock()
	s.queue.Push(agent.Message{Text: "pending"})
	assertBusy(true)
	s.queue.Take()
	s.queue.Push(agent.Message{Viewed: true})
	assertBusy(true)
	s.queue.Take()
	task, err := e.tasks.Start(process.StartOptions{Session: "unloaded-session", Owner: "test", Command: "sleep 30", Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	assertBusy(true)
	if _, err := e.tasks.Cancel("unloaded-session", task.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := e.tasks.Wait(ctx, "unloaded-session", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	for {
		busy, err := e.HasUpdateWork(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !busy {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
}
