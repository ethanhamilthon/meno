package engine

import (
	"meno/internal/agent"
)

// haltFor ends a subagent's run after the tool batch in which it asked to be
// blocked; the parent then hears about it as an event.
func (e *Engine) haltFor(s *runState) func(bool) bool {
	if s.session.Kind != agent.KindChild {
		return nil
	}
	return func(bool) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.block != ""
	}
}

// waitingSubagents reports that the session has subagents still working, so a
// goal waits for their events instead of nudging the agent.
func (e *Engine) waitingSubagents(sid string) bool {
	recs, err := e.children.List(sid, true)
	if err != nil {
		return false
	}
	for _, r := range recs {
		if r.Status == agent.ChildStarting || r.Status == agent.ChildRunning {
			return true
		}
	}
	return false
}
