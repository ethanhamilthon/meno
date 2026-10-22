package process

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

func newID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parse(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func (s *Supervisor) save(t Task) error {
	var code any
	if t.ExitCode != nil {
		code = *t.ExitCode
	}
	if _, err := s.d.Exec("INSERT OR IGNORE INTO sessions(id,updated_at) VALUES(?,?)", t.Session, ts(time.Now())); err != nil {
		return err
	}
	_, err := s.d.Exec(`INSERT INTO tasks(id,session_id,owner,name,command,cwd,status,exit_code,reason,notify,timeout_ms,started_at,ended_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET status=excluded.status, exit_code=excluded.exit_code, reason=excluded.reason, ended_at=excluded.ended_at`,
		t.ID, t.Session, t.Owner, t.Name, t.Command, t.Cwd, t.Status, code, t.Reason, b2i(t.Notify), 0, ts(t.Started), ts(t.Ended))
	return err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

const taskCols = "id,session_id,owner,name,command,cwd,status,exit_code,reason,notify,timeout_ms,started_at,ended_at"

func scanTask(sc interface{ Scan(...any) error }) (Task, error) {
	var t Task
	var code sql.NullInt64
	var notify int
	var ms int64
	var started, ended string
	if err := sc.Scan(&t.ID, &t.Session, &t.Owner, &t.Name, &t.Command, &t.Cwd, &t.Status, &code, &t.Reason, &notify, &ms, &started, &ended); err != nil {
		return t, err
	}
	if code.Valid {
		c := int(code.Int64)
		t.ExitCode = &c
	}
	t.Notify = notify != 0
	_ = ms
	t.Started, t.Ended = parse(started), parse(ended)
	return t, nil
}

// Get returns a task of a session. A task of another session is not found:
// IDs do not grant access across sessions.
func (s *Supervisor) Get(session, id string) (Task, error) {
	t, err := scanTask(s.d.QueryRow("SELECT "+taskCols+" FROM tasks WHERE id = ? AND session_id = ?", id, session))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, fmt.Errorf("task %q not found", id)
	}
	return t, err
}

// List returns the tasks of a session, oldest first.
func (s *Supervisor) List(session string, includeFinished bool) ([]Task, error) {
	q := "SELECT " + taskCols + " FROM tasks WHERE session_id = ?"
	if !includeFinished {
		q += " AND status IN ('starting','running')"
	}
	rows, err := s.d.Query(q+" ORDER BY started_at, id", session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Supervisor) markInterrupted() error {
	_, err := s.d.Exec(`UPDATE tasks SET status='interrupted', reason='meno exited while the task was running', ended_at=?
		WHERE status IN ('starting','running')`, ts(time.Now()))
	return err
}
