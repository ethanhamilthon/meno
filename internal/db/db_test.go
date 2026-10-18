package db

import (
	"database/sql"
	"path/filepath"
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

func TestMigrationsReachCurrentVersion(t *testing.T) {
	if len(migrations) != Version {
		t.Fatalf("%d migrations for version %d", len(migrations), Version)
	}
	d := rawDB(t)
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	if v := version(t, d); v != Version {
		t.Fatalf("version %d", v)
	}
	if err := migrate(d); err != nil {
		t.Fatalf("second run must be a no-op: %v", err)
	}
}

// Data written by an older meno survives every later migration.
func TestMigrationsKeepExistingData(t *testing.T) {
	d := rawDB(t)
	if err := migrateTo(d, 2); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"INSERT INTO settings(key, value_json) VALUES ('theme', '\"dracula\"')",
		"INSERT INTO providers(pos, name, base_url, api_key, protocol) VALUES (0, 'p', 'u', 'k', 'chat')",
		"INSERT INTO sessions(id, title, cwd, updated_at) VALUES ('s1', 'old', '/w', '2025-01-01T00:00:00Z')",
		"INSERT INTO turns(session_id, seq, role, text, time) VALUES ('s1', 0, 'user', 'hello', '2025-01-01T00:00:00Z')",
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	var theme, key, title, text string
	checks := []struct {
		q    string
		dest *string
		want string
	}{
		{"SELECT value_json FROM settings WHERE key='theme'", &theme, `"dracula"`},
		{"SELECT api_key FROM providers WHERE name='p'", &key, "k"},
		{"SELECT title FROM sessions WHERE id='s1'", &title, "old"},
		{"SELECT text FROM turns WHERE session_id='s1' AND seq=0", &text, "hello"},
	}
	for _, c := range checks {
		if err := d.QueryRow(c.q).Scan(c.dest); err != nil || *c.dest != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.q, *c.dest, err, c.want)
		}
	}
}

func TestNewerSchemaIsRefused(t *testing.T) {
	d := rawDB(t)
	if _, err := d.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(d); err == nil {
		t.Fatal("a database from a newer meno must be refused")
	}
	if v := version(t, d); v != 99 {
		t.Fatalf("refused database was changed: %d", v)
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	d := rawDB(t)
	saved := migrations
	defer func() { migrations = saved }()
	migrations = append(append([]string{}, saved[:2]...), "CREATE TABLE half(x); THIS IS NOT SQL;")
	if err := migrateTo(d, 3); err == nil {
		t.Fatal("expected an error")
	}
	if v := version(t, d); v != 2 {
		t.Fatalf("version %d after failed step", v)
	}
	var n int
	if err := d.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='half'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial migration left behind: %d %v", n, err)
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
