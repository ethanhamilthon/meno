package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func rawDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "x.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	return d
}

func version(t *testing.T, d *sql.DB) int {
	t.Helper()
	var v int
	if err := d.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFreshDatabaseIsAtTheCurrentVersion(t *testing.T) {
	d := rawDB(t)
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	if v := version(t, d); v != Version {
		t.Fatalf("version %d, want %d", v, Version)
	}
	for _, gone := range []string{"providers", "mcp_servers"} {
		var n int
		_ = d.QueryRow("SELECT COUNT(1) FROM sqlite_master WHERE name = ?", gone).Scan(&n)
		if n != 0 {
			t.Fatalf("table %s must not exist", gone)
		}
	}
	if err := migrate(d); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

// A 0.7.5 database (schema 19) still has the unused tables; the migration drops them.
func TestVersion19DropsUnusedTablesAndKeepsSessions(t *testing.T) {
	d := rawDB(t)
	if _, err := d.Exec(baseline + "CREATE TABLE providers (name TEXT); CREATE TABLE mcp_servers (name TEXT);" +
		"ALTER TABLE sessions ADD COLUMN spawned_by TEXT NOT NULL DEFAULT '';" +
		"INSERT INTO meta(key,value) VALUES ('format_epoch','0.5'),('migrated_sessions_db','1');" +
		"INSERT INTO sessions(id, cwd, updated_at) VALUES ('s1', '/x', '1'); PRAGMA user_version = 19;"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	var n, gone int
	_ = d.QueryRow("SELECT COUNT(1) FROM sessions WHERE id = 's1'").Scan(&n)
	_ = d.QueryRow("SELECT COUNT(1) FROM sqlite_master WHERE name IN ('providers','mcp_servers')").Scan(&gone)
	var cols, markers int
	_ = d.QueryRow("SELECT COUNT(1) FROM pragma_table_info('sessions') WHERE name = 'spawned_by'").Scan(&cols)
	_ = d.QueryRow("SELECT COUNT(1) FROM meta").Scan(&markers)
	if cols != 0 || markers != 0 {
		t.Fatalf("spawned_by columns %d, meta rows %d", cols, markers)
	}
	if n != 1 || gone != 0 || version(t, d) != Version {
		t.Fatalf("sessions %d, unused tables %d, version %d", n, gone, version(t, d))
	}
}

func TestOldDatabaseIsRefused(t *testing.T) {
	d := rawDB(t)
	if _, err := d.Exec("PRAGMA user_version = 18"); err != nil {
		t.Fatal(err)
	}
	err := migrate(d)
	if err == nil || !strings.Contains(err.Error(), "0.7.6") {
		t.Fatalf("an old database must be refused with a way forward: %v", err)
	}
}

func TestNewerSchemaIsRefused(t *testing.T) {
	d := rawDB(t)
	if _, err := d.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(d); err == nil {
		t.Fatal("a newer schema must be refused")
	}
}

func TestOpenSharesHandleAndSecuresFile(t *testing.T) {
	home := t.TempDir()
	a, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(home)
	if err != nil || a != b {
		t.Fatalf("second Open must return the shared handle: %v", err)
	}
}

func TestCloseAllReleasesTheSharedHandles(t *testing.T) {
	home := t.TempDir()
	a, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	CloseAll()
	if err := a.Ping(); err == nil {
		t.Fatal("CloseAll must close the handle")
	}
	b, err := Open(home)
	if err != nil || b == a {
		t.Fatalf("Open after CloseAll must make a new handle: %v", err)
	}
	CloseAll()
}

// A migration that fails changes nothing: the version stays at the last good step.
func TestFailedMigrationRollsBack(t *testing.T) {
	saved := migrations
	migrations = append(append([]string{}, migrations...), "CREATE TABLE half (x); NOT SQL")
	defer func() { migrations = saved }()
	d := rawDB(t)
	if _, err := d.Exec(baseline + "PRAGMA user_version = 20"); err != nil {
		t.Fatal(err)
	}
	if err := migrateTo(d, baselineVersion+len(migrations)); err == nil {
		t.Fatal("a broken migration must fail")
	}
	var n int
	_ = d.QueryRow("SELECT COUNT(1) FROM sqlite_master WHERE name = 'half'").Scan(&n)
	if n != 0 || version(t, d) != 20 {
		t.Fatalf("half-applied migration: table %d, version %d", n, version(t, d))
	}
}
