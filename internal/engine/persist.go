package engine

import (
	"errors"
	"meno/internal/agent"
	"meno/internal/proto"
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
		e.announce(proto.EventSessionCreated, s.session.ID, "")
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
	e.announce(proto.EventSessionCreated, s.session.ID, "")
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
	return st, nil
}

// adopt registers a new, unsaved session. It lives in memory until its first
// message; nothing is written and nobody is told about it yet.
func (e *Engine) adopt(sess *agent.Session) (*runState, error) {
	st, err := e.newState(sess, false)
	if err != nil {
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
