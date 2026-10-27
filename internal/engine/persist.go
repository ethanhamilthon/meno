package engine

import (
	"errors"
	"meno/internal/agent"
	"meno/internal/api"
	"time"
)

// hasContent reports whether the session holds anything worth keeping: a
// message, a compaction summary or a queued message. A session without any is
// only a place to type in, and it is not stored.
func hasContent(s *runState) bool {
	return s.session.Len() > 0 || s.session.Summary != "" || s.queue.Len() > 0
}

// save writes the session once it has content. The first write is what creates
// the session for everyone: it is announced then, not when it was opened. The
// caller holds s.mu.
func (e *Engine) save(s *runState) error {
	if !s.persisted && !hasContent(s) {
		return nil
	}
	if err := agent.SaveSession(e.home, s.session); err != nil {
		return err
	}
	if !s.persisted {
		s.persisted = true
		e.announce(api.EventSessionCreated, s.session.ID, "")
	}
	return nil
}

// ensureSaved stores the session even if it is still empty, for work that
// refers to it in the database (a background task). The caller holds s.mu.
func (e *Engine) ensureSaved(s *runState) error {
	if s.persisted {
		return nil
	}
	if err := agent.SaveSession(e.home, s.session); err != nil {
		return err
	}
	s.persisted = true
	e.announce(api.EventSessionCreated, s.session.ID, "")
	return nil
}

// newState wraps a session for the engine. persisted tells whether it already
// has a row in the database.
func (e *Engine) newState(sess *agent.Session, persisted bool) (*runState, error) {
	sess.AttachDir = e.paths.File("attachments")
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	st := &runState{persisted: persisted, cancelled: map[string]bool{}, session: sess, cwd: sess.Cwd, env: map[string]string{}, changed: time.Now(), token: tokenState{value: tok, groups: groupsFor(nil)}}
	st.published = len(sess.Snapshot())
	st.visibleTurns = wireTurns(sess.Snapshot())
	return st, nil
}

// adopt registers a new, unsaved session. It lives in memory until its first
// message; nothing is written and nobody is told about it yet.
func (e *Engine) adopt(sess *agent.Session) (*runState, error) {
	if err := e.lockSession(sess.ID); err != nil {
		return nil, err
	}
	st, err := e.newState(sess, false)
	if err != nil {
		e.unlockSession(sess.ID)
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, errors.New("engine is closed")
	}
	e.sessions[sess.ID] = st
	return st, nil
}

// cloneLive copies the live state so that a snapshot encoded after the lock is
// released does not share the call list the run keeps changing in place.
func cloneLive(l api.LiveState) api.LiveState {
	l.Calls = append([]api.CallState{}, l.Calls...)
	for i := range l.Calls {
		if c := l.Calls[i].Exit; c != nil {
			v := *c
			l.Calls[i].Exit = &v
		}
	}
	return l
}

// cloneRetry copies the retry state for the same reason.
func cloneRetry(r *api.RetryState) *api.RetryState {
	if r == nil {
		return nil
	}
	v := *r
	return &v
}
