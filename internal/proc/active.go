package proc

// HasActive reports work across every session, including unloaded sessions.
// A database failure must not be interpreted as permission to stop the daemon.
func (s *Supervisor) HasActive() (bool, error) {
	if s.outstanding.Load() > 0 {
		return true, nil
	}
	var n int
	err := s.d.QueryRow("SELECT COUNT(*) FROM tasks WHERE status IN ('starting','running')").Scan(&n)
	return n > 0 || s.outstanding.Load() > 0, err
}

// admit reserves a supervisor slot unless shutdown has begun. The check and the
// wg.Add share one lock with Shutdown's closing flag, so a process is either
// refused or counted before Shutdown can finish waiting.
func (s *Supervisor) admit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.wg.Add(1)
	s.outstanding.Add(1)
	return true
}

// release keeps completion callbacks in the active-work boundary as well.
func (s *Supervisor) release() {
	s.outstanding.Add(-1)
	s.wg.Done()
}
