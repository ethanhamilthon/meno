package process

import (
	"context"
	"time"
)

// Cancel stops one task. It is idempotent and does nothing to a finished one.
func (s *Supervisor) Cancel(session, id string) (Task, error) {
	t, err := s.Get(session, id)
	if err != nil {
		return t, err
	}
	if r := s.liveTask(id); r != nil {
		s.mu.Lock()
		first := !r.cancelled
		r.cancelled = true
		s.mu.Unlock()
		if first {
			s.terminate(r)
		}
	}
	return t, nil
}

// Wait blocks until the task ends or the timeout passes (0 waits as long as
// it takes, or until ctx ends). A timeout never cancels the task. waitTimedOut
// is true when the task was still running.
func (s *Supervisor) Wait(ctx context.Context, session, id string, timeout time.Duration) (t Task, waitTimedOut bool, err error) {
	// liveTask first: a task that is gone from live has its final status stored,
	// so the Get below cannot see a stale "running".
	r := s.liveTask(id)
	if t, err = s.Get(session, id); err != nil {
		return t, false, err
	}
	if r == nil {
		return t, false, nil
	}
	s.mu.Lock()
	s.waiters[id]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.waiters[id]--
		s.mu.Unlock()
	}()
	var expire <-chan time.Time
	if timeout > 0 {
		tm := time.NewTimer(timeout)
		defer tm.Stop()
		expire = tm.C
	}
	select {
	case <-r.done:
	case <-expire:
		t, err = s.Get(session, id)
		return t, true, err
	case <-ctx.Done():
		return t, false, ctx.Err()
	}
	t, err = s.Get(session, id)
	return t, false, err
}

// Logs reads the log of a task.
func (s *Supervisor) Logs(session, id string, o LogOptions) (LogSlice, error) {
	if _, err := s.Get(session, id); err != nil {
		return LogSlice{}, err
	}
	return ReadLogs(s.logPath(session, id), o), nil
}

// Shutdown cancels every running task, waits for the process groups to exit
// (after GraceTime they are killed) and joins the supervisors.
func (s *Supervisor) Shutdown() {
	s.mu.Lock()
	s.closing = true
	var rs []*running
	for _, r := range s.live {
		r.cancelled = true
		rs = append(rs, r)
	}
	s.mu.Unlock()
	for _, r := range rs {
		s.terminate(r)
	}
	s.wg.Wait()
}

// Waiting reports whether a Wait for the task is in progress right now.
func (s *Supervisor) Waiting(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.waiters[id] > 0
}
