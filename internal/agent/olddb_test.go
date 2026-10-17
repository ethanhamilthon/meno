package agent

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestImportsOldSessionsDB(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	dir := filepath.Join(home, "sessions")
	os.MkdirAll(dir, 0o755)
	old, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(schema); err != nil {
		t.Fatal(err)
	}
	old.Exec("INSERT INTO sessions(id,title,updated_at) VALUES('o1','old',?)", ts(time.Now()))
	old.Close()
	list, err := ListSessions(home)
	if err != nil || len(list) != 1 || list[0].ID != "o1" {
		t.Fatalf("%+v %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions.db")); err != nil {
		t.Fatal("old db must stay")
	}
}
