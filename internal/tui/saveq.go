package tui

import "sync"

// saveQueue runs saves to the daemon one at a time, in the order they were
// asked. Without it two quick edits race, and the older one can land last.
type saveQueue struct {
	mu      sync.Mutex
	pending []func()
	busy    bool
}

// push adds a save; run starts the worker when it is idle.
func (a *App) pushSave(fn func()) {
	q := &a.saves
	q.mu.Lock()
	q.pending = append(q.pending, fn)
	if q.busy {
		q.mu.Unlock()
		return
	}
	q.busy = true
	q.mu.Unlock()
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		for {
			q.mu.Lock()
			if len(q.pending) == 0 {
				q.busy = false
				q.mu.Unlock()
				return
			}
			next := q.pending[0]
			q.pending = q.pending[1:]
			q.mu.Unlock()
			next()
		}
	}()
}
