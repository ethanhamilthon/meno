package agent

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestOldSessionsDBIsNotImported(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	dir := filepath.Join(home, "sessions")
	os.MkdirAll(dir, 0o755)
	old, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	old.Exec("INSERT INTO sessions(id,title,updated_at) VALUES('o1','old',?)", ts(time.Now()))
	old.Close()
	list, err := ListSessions(home)
	if err != nil || len(list) != 0 {
		t.Fatalf("an old sessions.db must not be imported: %v %v", list, err)
	}
}

const oldSchema = `
CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', cwd TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '', metrics TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL);
CREATE TABLE turns (session_id TEXT NOT NULL, seq INTEGER NOT NULL, role TEXT NOT NULL, text TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '',
	call_id TEXT NOT NULL DEFAULT '', code TEXT NOT NULL DEFAULT '', output TEXT NOT NULL DEFAULT '', err TEXT NOT NULL DEFAULT '', replay TEXT,
	replay_group TEXT NOT NULL DEFAULT '', time TEXT NOT NULL, PRIMARY KEY (session_id, seq));
CREATE TABLE requests (session_id TEXT NOT NULL, seq INTEGER NOT NULL, provider TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
	usage TEXT NOT NULL, cost REAL, time TEXT NOT NULL, PRIMARY KEY (session_id, seq));
`
