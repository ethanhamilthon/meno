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
	if _, err := d.Exec("INSERT INTO sessions(id,updated_at) VALUES('old','2026-01-01')"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO requests(session_id,seq,provider,model,usage,cost,time) VALUES('old',0,'p','m','{\"In\":42}',1.25,'2026-01-01')"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(d); err != nil {
		t.Fatalf("upgrade from version %d: %v", Version-1, err)
	}
	if _, err := d.Exec("SELECT tool FROM sessions LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("SELECT details FROM requests LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("SELECT id,data FROM request_configs LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("SELECT args_json,cwd FROM mcp_servers LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	var usage, details string
	var cost float64
	if err := d.QueryRow("SELECT usage,cost,details FROM requests WHERE session_id='old'").Scan(&usage, &cost, &details); err != nil {
		t.Fatal(err)
	}
	if usage != `{"In":42}` || cost != 1.25 || details != "" {
		t.Fatalf("legacy row changed: %s %v %s", usage, cost, details)
	}
}
