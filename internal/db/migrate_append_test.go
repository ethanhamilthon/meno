package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// A database at the previous schema version must upgrade: new migrations go at
// the end of the list, never in the middle.
func TestMigrationsUpgradeFromThePreviousVersion(t *testing.T) {
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := migrateTo(d, Version-1); err != nil {
		t.Fatal(err)
	}
	if err := migrate(d); err != nil {
		t.Fatalf("upgrade from version %d: %v", Version-1, err)
	}
	if _, err := d.Exec("SELECT tool FROM sessions LIMIT 1"); err != nil {
		t.Fatal(err)
	}
}
