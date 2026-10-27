package agent

import (
	"database/sql"
	"sync"
	"time"

	"meno/internal/db"
)

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
	sessInit[d] = true
	return d, nil
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
