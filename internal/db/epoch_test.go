package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestEpochBackupIncludesWALAndResetsOnce(t *testing.T) {
	home := t.TempDir()
	old, err := OpenOld(home, 12)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = old.Exec("INSERT INTO meta(key,value)VALUES('private','preserved')"); err != nil {
		t.Fatal(err)
	}
	backup, err := PrepareEpoch(home)
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	if backup == "" {
		t.Fatal("no backup")
	}
	info, err := os.Stat(backup)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	saved, err := sql.Open("sqlite", "file:"+backup+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	var value string
	if err = saved.QueryRow("SELECT value FROM meta WHERE key='private'").Scan(&value); err != nil || value != "preserved" {
		t.Fatal(value, err)
	}
	fresh, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseAll()
	if err = fresh.QueryRow("SELECT value FROM meta WHERE key='private'").Scan(&value); err != sql.ErrNoRows {
		t.Fatal("old data survived reset", err)
	}
	if second, err := PrepareEpoch(home); err != nil || second != "" {
		t.Fatal("reset twice", second, err)
	}
	var n int
	if err = fresh.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='snippets'").Scan(&n); err != nil || n != 0 {
		t.Fatal("snippet schema remains")
	}
}

func TestEpochCorruptDatabaseIsNotDestroyed(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, File)
	original := []byte("not sqlite")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareEpoch(home); err == nil {
		t.Fatal("corrupt database accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("corrupt source changed")
	}
}

func TestEpochBackupFailureKeepsOriginal(t *testing.T) {
	home := t.TempDir()
	d, err := OpenOld(home, 12)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	if err = os.WriteFile(filepath.Join(home, "backups"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareEpoch(home); err == nil {
		t.Fatal("backup failure ignored")
	}
	check, err := sql.Open("sqlite", "file:"+filepath.Join(home, File))
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var v int
	if err = check.QueryRow("PRAGMA user_version").Scan(&v); err != nil || v != 12 {
		t.Fatal("old database altered", v, err)
	}
}
