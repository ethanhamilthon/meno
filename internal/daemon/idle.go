package daemon

import (
	"context"
	"time"
)

const idleExitDefault = 10 * time.Second

func (s *Server) touchIdle() { s.idleMu.Lock(); s.lastActive = time.Now(); s.idleMu.Unlock() }

func (s *Server) hasCompleteWork(ctx context.Context) (bool, error) {
	if b, ok := s.backend.(interface {
		HasUpdateWork(context.Context) (bool, error)
	}); ok {
		return b.HasUpdateWork(ctx)
	}
	return s.backend.HasWork(ctx), nil
}

// checkIdle commits shutdown under the same admission boundary as requests and
// stream registration. Stop itself runs outside the lock, allowing teardown.
func (s *Server) checkIdle(ctx context.Context, now time.Time) bool {
	s.admission.Lock()
	defer s.admission.Unlock()
	if s.stopping || ctx.Err() != nil {
		return false
	}
	if s.requests > 0 || s.hub.Subscribers() > 0 || s.global.Subscribers() > 0 {
		s.idleMu.Lock()
		s.lastActive = now
		s.idleMu.Unlock()
		return false
	}
	busy, err := s.hasCompleteWork(ctx)
	s.idleMu.Lock()
	defer s.idleMu.Unlock()
	if s.requests > 0 || err != nil || busy || s.hub.Subscribers() > 0 || s.global.Subscribers() > 0 {
		s.lastActive = now
		return false
	}
	if now.Sub(s.lastActive) < s.idle {
		return false
	}
	s.stopping = true
	return true
}

func (s *Server) idleLoop(ctx context.Context) {
	tick := time.NewTicker(min(time.Second, max(time.Millisecond, s.idle/4)))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case now := <-tick.C:
			if s.checkIdle(ctx, now) {
				s.Stop("idle")
				return
			}
		}
	}
}
