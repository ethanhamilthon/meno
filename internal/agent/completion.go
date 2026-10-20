package agent

import (
	"errors"
	"time"
)

// SaveCompletion commits final run history and its unread generation atomically.
func SaveCompletion(home string, s *Session) error {
	if s.ID == "" {
		return errors.New("session id is empty")
	}
	s.UpdatedAt = time.Now()
	s.Turns = s.Snapshot()
	if s.Title == "" || s.Title == "untitled" {
		s.Title = deriveTitle(s)
	}
	d, err := openDB(home)
	if err != nil {
		return err
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pending, err := writeSessionTx(tx, s)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO session_attention(session_id,completed) VALUES(?,1)
 ON CONFLICT(session_id) DO UPDATE SET completed=completed+1`, s.ID)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.ensureLog().ClearPending(len(pending.Archive), len(pending.Summaries))
	return nil
}
