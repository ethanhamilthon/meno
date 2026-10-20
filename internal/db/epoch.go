package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PrepareEpoch must run under daemon startup ownership, before services open DBs.
// It preserves a consistent SQLite backup before activating the clean 0.5 format.
func PrepareEpoch(home string) (string, error) {
	path := filepath.Join(home, File)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	old, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return "", err
	}
	defer old.Close()
	var epoch string
	_ = old.QueryRow("SELECT value FROM meta WHERE key='format_epoch'").Scan(&epoch)
	if epoch != "" && epoch != "0.5" {
		return "", fmt.Errorf("unsupported database format epoch %q", epoch)
	}
	if epoch == "0.5" {
		return "", nil
	}
	var integrity string
	if err = old.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return "", err
	}
	if integrity != "ok" {
		return "", fmt.Errorf("database integrity check failed: %s", integrity)
	}
	dir := filepath.Join(home, "backups")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	backup := filepath.Join(dir, "meno-pre-0.5-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
	f, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	f.Close()
	// VACUUM INTO requires an absent or empty destination and includes WAL commits.
	if _, err = old.Exec("VACUUM INTO '" + strings.ReplaceAll(backup, "'", "''") + "'"); err != nil {
		return "", fmt.Errorf("backup %s: %w", backup, err)
	}
	check, err := sql.Open("sqlite", "file:"+backup+"?mode=ro")
	if err != nil {
		return backup, err
	}
	err = check.QueryRow("PRAGMA integrity_check").Scan(&integrity)
	check.Close()
	if err != nil || integrity != "ok" {
		return backup, fmt.Errorf("backup integrity check failed: %v %s", err, integrity)
	}
	f, err = os.OpenFile(backup, os.O_RDWR, 0600)
	if err != nil {
		return backup, err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return backup, err
	}
	staging, err := os.MkdirTemp(home, ".epoch-")
	if err != nil {
		return backup, err
	}
	defer os.RemoveAll(staging)
	fresh, err := OpenOld(staging, Version)
	if err != nil {
		return backup, err
	}
	_, err = fresh.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	fresh.Close()
	if err != nil {
		return backup, err
	}
	if _, err = old.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return backup, err
	}
	if err = old.Close(); err != nil {
		return backup, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err = os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			return backup, err
		}
	}
	if err = os.Chmod(filepath.Join(staging, File), 0600); err != nil {
		return backup, err
	}
	freshFile, openErr := os.OpenFile(filepath.Join(staging, File), os.O_RDWR, 0600)
	if openErr != nil {
		return backup, openErr
	}
	syncErr := freshFile.Sync()
	freshFile.Close()
	if syncErr != nil {
		return backup, syncErr
	}
	if err = os.Rename(filepath.Join(staging, File), path); err != nil {
		return backup, err
	}
	d, err := os.Open(home)
	if err == nil {
		err = d.Sync()
		d.Close()
	}
	return backup, err
}
