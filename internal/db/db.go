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
const Version = 20

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

func migrate(d *sql.DB) error { return migrateTo(d, Version) }

func migrateTo(d *sql.DB, target int) error {
	var v int
	if err := d.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	switch {
	case v > Version:
		return fmt.Errorf("database schema %d is newer than this meno (%d)", v, Version)
	case v == 0:
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(baseline); err == nil {
			_, err = tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", baselineVersion+len(migrations)))
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	case v < baselineVersion:
		return fmt.Errorf("meno.db was made by a meno older than 0.7.5 (schema %d). Install meno 0.7.6, run it once, then update", v)
	}
	for ; v < target; v++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v-baselineVersion]); err != nil {
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
