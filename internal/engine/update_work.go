package engine

import "context"

// HasUpdateWork is called with HTTP mutation admission closed. activeRuns also
// covers the interval between finishing a turn and launching queued work.
func (e *Engine) HasUpdateWork(context.Context) (bool, error) {
	e.mu.Lock()
	all := make([]*runState, 0, len(e.sessions))
	for _, s := range e.sessions {
		all = append(all, s)
	}
	e.mu.Unlock()
	for _, s := range all {
		s.mu.Lock()
		busy := s.running || s.ask != nil || s.compacting || s.queue.Len() > 0 || s.queue.Viewed() > 0
		s.mu.Unlock()
		if busy {
			return true, nil
		}
	}
	// Hold run admission while checking supervisor ownership. Completion hooks
	// stay counted until they finish launching continuations, so a task-to-run
	// handoff cannot disappear between two independent observations.
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeRuns > 0 {
		return true, nil
	}
	return e.tasks.HasActive()
}
