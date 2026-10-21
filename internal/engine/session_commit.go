package engine

import "meno/internal/proto"

// bootIDer is implemented by event hubs that know the daemon boot identity.
type bootIDer interface{ BootID() string }

// version is the compare boundary of a session: the event sequence is assigned
// at publication, so it moves with every committed change within one boot.
func (e *Engine) version(seq uint64) proto.SessionVersion {
	v := proto.SessionVersion{Revision: seq}
	if b, ok := e.events.(bootIDer); ok {
		v.BootID = b.BootID()
	}
	return v
}

// currentRequest returns the request ID of the run the session is executing.
func (e *Engine) currentRequest(id string) string {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requestID
}
