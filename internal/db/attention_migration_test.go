package db

import "testing"

func TestMigrationHidesExistingUnreadCompletions(t *testing.T) {
	d := rawDB(t)
	if err := migrateTo(d, Version-2); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"INSERT INTO sessions(id,title,cwd,updated_at) VALUES('unread','','',''),('read','','','')",
		"INSERT INTO session_attention VALUES('unread',5,2),('read',3,3)",
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want int
	}{{"unread", 5}, {"read", 3}} {
		var completed, read int
		if err := d.QueryRow("SELECT completed,read_through FROM session_attention WHERE session_id=?", tc.id).Scan(&completed, &read); err != nil {
			t.Fatal(err)
		}
		if completed != tc.want || read != tc.want {
			t.Fatalf("%s = %d/%d", tc.id, completed, read)
		}
	}
}
