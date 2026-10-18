package agent

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"meno/internal/db"
	"meno/internal/store"
)

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
	if err := ensureColumns(d); err != nil {
		return nil, fmt.Errorf("session db: %w", err)
	}
	sessInit[d] = true
	legacy := filepath.Join(home, "sessions")
	importOldDB(d, filepath.Join(legacy, "sessions.db"))
	migrateLegacy(d, legacy)
	// Summaries of imported legacy sessions become revision 0.
	_, _ = d.Exec(`INSERT OR IGNORE INTO summaries(session_id,rev,summary,covers_through,created_at)
		SELECT id, 0, summary, 0, updated_at FROM sessions WHERE legacy = 1 AND summary <> ''`)
	return d, nil
}

// ensureColumns adds columns that tables created by older versions lack:
// turns.system (system events) and sessions.provider / sessions.effort (a model
// per session).
func ensureColumns(d *sql.DB) error {
	for _, c := range []struct{ table, name, def string }{
		{"turns", "system", "TEXT NOT NULL DEFAULT ''"},
		{"turns", "attachments", "TEXT NOT NULL DEFAULT ''"},
		{"turns", "kind", "TEXT NOT NULL DEFAULT ''"},
		{"turns", "elapsed_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"turns", "limit_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"turns", "timed_out", "INTEGER NOT NULL DEFAULT 0"},
		{"turns", "run", "TEXT NOT NULL DEFAULT ''"},
		{"sessions", "provider", "TEXT NOT NULL DEFAULT ''"},
		{"sessions", "effort", "TEXT NOT NULL DEFAULT ''"},
	} {
		has, err := hasColumn(d, c.table, c.name)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := d.Exec("ALTER TABLE " + c.table + " ADD COLUMN " + c.name + " " + c.def); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(d *sql.DB, table, column string) (bool, error) {
	rows, err := d.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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
		"INSERT OR IGNORE INTO sessions(id,title,cwd,model,summary,metrics,updated_at) SELECT id,title,cwd,model,summary,metrics,updated_at FROM old.sessions",
		"INSERT OR IGNORE INTO turns(session_id,seq,role,text,thinking,call_id,code,output,err,replay,replay_group,time) SELECT session_id,seq,role,text,thinking,call_id,code,output,err,replay,replay_group,time FROM old.turns",
		"INSERT OR IGNORE INTO requests(session_id,seq,provider,model,usage,cost,time) SELECT session_id,seq,provider,model,usage,cost,time FROM old.requests",
		"UPDATE sessions SET legacy = 1, coverage = CASE WHEN summary <> '' THEN 'partial' ELSE coverage END WHERE legacy = 0 AND id IN (SELECT id FROM old.sessions)",
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
			s.Legacy = true
			if s.Summary != "" {
				s.Coverage = CoveragePartial
			}
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
