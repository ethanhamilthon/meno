// Package db opens the single SQLite database (meno.db) that holds settings,
// providers, MCP servers, skill state, the model catalog and all sessions.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // pure-Go driver, no CGO
)

// File is the database file name inside the meno home directory.
const File = "meno.db"

// Version is the current schema version; see migrations.
const Version = 1

// migrations[i] upgrades the schema from version i to i+1.
var migrations = []string{`
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE settings (
	key        TEXT PRIMARY KEY,
	value_json TEXT NOT NULL
);
CREATE TABLE providers (
	pos      INTEGER PRIMARY KEY,
	name     TEXT NOT NULL UNIQUE,
	base_url TEXT NOT NULL DEFAULT '',
	api_key  TEXT NOT NULL DEFAULT '',
	protocol TEXT NOT NULL DEFAULT ''
);
CREATE TABLE mcp_servers (
	pos          INTEGER PRIMARY KEY,
	name         TEXT NOT NULL UNIQUE,
	transport    TEXT NOT NULL DEFAULT '',
	command      TEXT NOT NULL DEFAULT '',
	url          TEXT NOT NULL DEFAULT '',
	enabled      INTEGER NOT NULL DEFAULT 1,
	env_json     TEXT NOT NULL DEFAULT '{}',
	headers_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE skills_state (
	name    TEXT PRIMARY KEY,
	enabled INTEGER NOT NULL
);
CREATE TABLE model_catalog (
	model TEXT PRIMARY KEY,
	data  TEXT NOT NULL
);
`}

var (
	mu    sync.Mutex
	conns = map[string]*sql.DB{}
)

// Open returns the shared handle for home/meno.db, creating and migrating it.
func Open(home string) (*sql.DB, error) {
	path := filepath.Join(home, File)
	mu.Lock()
	defer mu.Unlock()
	if d, ok := conns[path]; ok {
		return d, nil
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	dsn := u.String() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection serializes writers inside the process; other meno
	// processes are handled by SQLite locking plus busy_timeout.
	d.SetMaxOpenConns(1)
	if err := migrate(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("meno.db: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		d.Close()
		return nil, err
	}
	conns[path] = d
	return d, nil
}

func migrate(d *sql.DB) error {
	var v int
	if err := d.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v > Version {
		return fmt.Errorf("database schema %d is newer than this meno (%d)", v, Version)
	}
	for ; v < Version; v++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// CloseAll closes every opened database (flushing the WAL).
func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for p, d := range conns {
		_ = d.Close()
		delete(conns, p)
	}
}
