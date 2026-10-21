package proc

import (
	"sync"
	"testing"
	"time"
)

// Start racing with Shutdown: a process that was admitted must be supervised
// (and so cancelled and joined), and one that was refused must not run.
func TestStartRacingShutdownNeverLeavesAnUnsupervisedProcess(t *testing.T) {
	for i := 0; i < 40; i++ {
		s, _ := newSup(t, nil)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Start(StartOptions{Session: "s", Owner: "o", Command: "sleep 30", Cwd: t.TempDir()})
		}()
		time.Sleep(time.Duration(i%5) * time.Millisecond)
		done := make(chan struct{})
		go func() { s.Shutdown(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("iteration %d: Shutdown did not finish", i)
		}
		wg.Wait()
		s.mu.Lock()
		live := len(s.live)
		s.mu.Unlock()
		if live != 0 {
			t.Fatalf("iteration %d: %d process(es) still registered after Shutdown", i, live)
		}
	}
}
