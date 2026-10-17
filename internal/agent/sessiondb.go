package agent

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/store"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL DEFAULT '',
	cwd        TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	summary    TEXT NOT NULL DEFAULT '',
	metrics    TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_updated ON sessions(updated_at DESC);
CREATE TABLE IF NOT EXISTS turns (
	session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	seq          INTEGER NOT NULL,
	role         TEXT NOT NULL,
	text         TEXT NOT NULL DEFAULT '',
	thinking     TEXT NOT NULL DEFAULT '',
	call_id      TEXT NOT NULL DEFAULT '',
	code         TEXT NOT NULL DEFAULT '',
	output       TEXT NOT NULL DEFAULT '',
	err          TEXT NOT NULL DEFAULT '',
	replay       TEXT,
	replay_group TEXT NOT NULL DEFAULT '',
	time         TEXT NOT NULL,
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS requests (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	seq        INTEGER NOT NULL,
	provider   TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	usage      TEXT NOT NULL,
	cost       REAL,
	time       TEXT NOT NULL,
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
`

// CloseSessionStores closes every opened database (flushing the WAL).
func CloseSessionStores() { db.CloseAll() }

var (
	sessMu   sync.Mutex
	sessInit = map[*sql.DB]bool{}
)

// openDB returns the shared handle for home/meno.db with the session tables
// created. On first use in this process it imports the old sessions/sessions.db
// and per-session JSON files.
func openDB(home string) (*sql.DB, error) {
	d, err := db.Open(home)
	if err != nil {
		return nil, err
	}
	sessMu.Lock()
	defer sessMu.Unlock()
	if sessInit[d] {
		return d, nil
	}
	if _, err := d.Exec(schema); err != nil {
		return nil, fmt.Errorf("session db: %w", err)
	}
	sessInit[d] = true
	legacy := filepath.Join(home, "sessions")
	importOldDB(d, filepath.Join(legacy, "sessions.db"))
	migrateLegacy(d, legacy)
	return d, nil
}

// importOldDB copies the sessions of the pre-meno.db database file once. The
// old file is kept.
func importOldDB(d *sql.DB, old string) {
	if _, err := os.Stat(old); err != nil {
		return
	}
	var n int
	_ = d.QueryRow("SELECT COUNT(1) FROM meta WHERE key='migrated_sessions_db'").Scan(&n)
	if n > 0 {
		return
	}
	// ATTACH is per connection and the pool holds exactly one.
	if _, err := d.Exec("ATTACH DATABASE ? AS old", "file:"+old+"?mode=ro"); err != nil {
		return
	}
	defer d.Exec("DETACH DATABASE old")
	tx, err := d.Begin()
	if err != nil {
		return
	}
	for _, q := range []string{
		"INSERT OR IGNORE INTO sessions SELECT * FROM old.sessions",
		"INSERT OR IGNORE INTO turns SELECT * FROM old.turns",
		"INSERT OR IGNORE INTO requests SELECT * FROM old.requests",
		"INSERT INTO meta(key,value) VALUES('migrated_sessions_db','1')",
	} {
		if _, err := tx.Exec(q); err != nil {
			_ = tx.Rollback()
			return
		}
	}
	_ = tx.Commit()
}

// migrateLegacy imports <id>.json files once and renames them to <id>.json.migrated
// so the originals are kept. Unreadable files are left in place untouched.
func migrateLegacy(d *sql.DB, dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" || strings.HasPrefix(name, ".") {
			continue
		}
		var s Session
		file := filepath.Join(dir, name)
		if err := store.LoadJSON(file, &s); err != nil || s.ID == "" {
			continue
		}
		var exists int
		_ = d.QueryRow("SELECT COUNT(1) FROM sessions WHERE id = ?", s.ID).Scan(&exists)
		if exists == 0 {
			if err := writeSession(d, &s); err != nil {
				continue
			}
		}
		_ = os.Rename(file, file+".migrated")
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func writeSession(db *sql.DB, s *Session) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	metrics, err := json.Marshal(s.Metrics)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO sessions(id,title,cwd,model,summary,metrics,updated_at) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET title=excluded.title, cwd=excluded.cwd, model=excluded.model,
		summary=excluded.summary, metrics=excluded.metrics, updated_at=excluded.updated_at`,
		s.ID, s.Title, s.Cwd, s.Model, s.Summary, string(metrics), ts(s.UpdatedAt)); err != nil {
		return err
	}
	// Turns may be rewritten wholesale (:clear, compaction), so replace them.
	if _, err = tx.Exec("DELETE FROM turns WHERE session_id = ?", s.ID); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM requests WHERE session_id = ?", s.ID); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO turns(session_id,seq,role,text,thinking,call_id,code,output,err,replay,replay_group,time)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for i, t := range s.Turns {
		var replay any
		if t.Replay != nil {
			b, merr := json.Marshal(t.Replay)
			if merr != nil {
				err = merr
				return err
			}
			replay = string(b)
		}
		if _, err = ins.Exec(s.ID, i, t.Role, t.Text, t.Thinking, t.CallID, t.Code, t.Output, t.Err, replay, t.ReplayGroup, ts(t.Time)); err != nil {
			return err
		}
	}
	req, err := tx.Prepare("INSERT INTO requests(session_id,seq,provider,model,usage,cost,time) VALUES(?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer req.Close()
	for i, r := range s.Requests {
		usage, merr := json.Marshal(r.Usage)
		if merr != nil {
			err = merr
			return err
		}
		var cost any
		if r.Cost != nil {
			cost = *r.Cost
		}
		if _, err = req.Exec(s.ID, i, r.Provider, r.Model, string(usage), cost, ts(r.Time)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readSession(db *sql.DB, id string) (*Session, error) {
	var s Session
	var metrics, updated string
	err := db.QueryRow("SELECT id,title,cwd,model,summary,metrics,updated_at FROM sessions WHERE id = ?", id).
		Scan(&s.ID, &s.Title, &s.Cwd, &s.Model, &s.Summary, &metrics, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	s.UpdatedAt = parseTS(updated)
	if err := json.Unmarshal([]byte(metrics), &s.Metrics); err != nil {
		return nil, fmt.Errorf("session %s metrics: %w", id, err)
	}
	rows, err := db.Query("SELECT role,text,thinking,call_id,code,output,err,replay,replay_group,time FROM turns WHERE session_id = ? ORDER BY seq", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Turn
		var replay sql.NullString
		var when string
		if err := rows.Scan(&t.Role, &t.Text, &t.Thinking, &t.CallID, &t.Code, &t.Output, &t.Err, &replay, &t.ReplayGroup, &when); err != nil {
			return nil, err
		}
		if replay.Valid {
			var m llm.Msg
			if err := json.Unmarshal([]byte(replay.String), &m); err != nil {
				return nil, fmt.Errorf("session %s replay: %w", id, err)
			}
			t.Replay = &m
		}
		t.Time = parseTS(when)
		s.Turns = append(s.Turns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rq, err := db.Query("SELECT provider,model,usage,cost,time FROM requests WHERE session_id = ? ORDER BY seq", id)
	if err != nil {
		return nil, err
	}
	defer rq.Close()
	for rq.Next() {
		var r RequestUsage
		var usage, when string
		var cost sql.NullFloat64
		if err := rq.Scan(&r.Provider, &r.Model, &usage, &cost, &when); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(usage), &r.Usage); err != nil {
			return nil, err
		}
		if cost.Valid {
			c := cost.Float64
			r.Cost = &c
		}
		r.Time = parseTS(when)
		s.Requests = append(s.Requests, r)
	}
	return &s, rq.Err()
}

func listSessions(db *sql.DB) ([]SessionInfo, error) {
	rows, err := db.Query(`SELECT s.id, s.title, s.cwd, s.updated_at,
		(SELECT COUNT(1) FROM turns t WHERE t.session_id = s.id),
		(SELECT COUNT(1) FROM turns t WHERE t.session_id = s.id AND (t.code <> '' OR t.call_id <> ''))
		FROM sessions s ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionInfo
	for rows.Next() {
		var i SessionInfo
		var updated string
		if err := rows.Scan(&i.ID, &i.Title, &i.Cwd, &updated, &i.Turns, &i.JSRuns); err != nil {
			return nil, err
		}
		i.UpdatedAt = parseTS(updated)
		out = append(out, i)
	}
	return out, rows.Err()
}
