package agent

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Namespaces of the versioned per-session stores.
const (
	NSData = "data"
)

// ErrLimit is returned when a write would exceed a size limit.
var ErrLimit = errors.New("size limit exceeded")

// KV is one versioned namespace of a session: the current values plus an
// append-only journal that lets any past boundary be reconstructed. Every
// mutation writes both in one transaction, so a successful call is durable.
type KV struct {
	d        *sql.DB
	session  string
	ns       string
	MaxValue int // bytes of one encoded value
	MaxTotal int // bytes of all current values
}

// Entry is one key with its encoded JSON value.
type Entry struct{ Key, JSON string }

// OpenKV returns the store of namespace ns for a session.
func OpenKV(dir, session, ns string) (*KV, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return &KV{d: d, session: session, ns: ns, MaxValue: 1 << 20, MaxTotal: 10 << 20}, nil
}

func (k *KV) tx(fn func(tx *sql.Tx) error) error {
	tx, err := k.d.Begin()
	if err != nil {
		return err
	}
	// The session row may not be saved yet; the journal needs it.
	if _, err := tx.Exec("INSERT OR IGNORE INTO sessions(id,updated_at) VALUES(?,?)", k.session, ts(time.Now())); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (k *KV) journal(tx *sql.Tx, key string, val any, deleted bool, boundary int64) error {
	var ver int64
	if err := tx.QueryRow("SELECT COALESCE(MAX(ver),0)+1 FROM journal WHERE session_id = ?", k.session).Scan(&ver); err != nil {
		return err
	}
	_, err := tx.Exec("INSERT INTO journal(session_id,ver,ns,key,value_json,deleted,boundary,time) VALUES(?,?,?,?,?,?,?,?)",
		k.session, ver, k.ns, key, val, b2i(deleted), boundary, ts(time.Now()))
	return err
}

// Set stores the encoded JSON value of key, recording boundary (the history ID
// the mutation happened at). Limits are checked before anything is written.
func (k *KV) Set(key, json string, boundary int64) error {
	if key == "" {
		return errors.New("key must not be empty")
	}
	if len(json) > k.MaxValue {
		return fmt.Errorf("%w: value of %q is %d bytes, the limit is %d", ErrLimit, key, len(json), k.MaxValue)
	}
	return k.tx(func(tx *sql.Tx) error {
		var total, old int
		if err := tx.QueryRow("SELECT COALESCE(SUM(LENGTH(value_json)),0) FROM kv WHERE session_id=? AND ns=?", k.session, k.ns).Scan(&total); err != nil {
			return err
		}
		_ = tx.QueryRow("SELECT LENGTH(value_json) FROM kv WHERE session_id=? AND ns=? AND key=?", k.session, k.ns, key).Scan(&old)
		if total-old+len(json) > k.MaxTotal {
			return fmt.Errorf("%w: the %s of this session would be %d bytes, the limit is %d", ErrLimit, k.ns, total-old+len(json), k.MaxTotal)
		}
		if _, err := tx.Exec(`INSERT INTO kv(session_id,ns,key,value_json) VALUES(?,?,?,?)
			ON CONFLICT(session_id,ns,key) DO UPDATE SET value_json=excluded.value_json`, k.session, k.ns, key, json); err != nil {
			return err
		}
		return k.journal(tx, key, json, false, boundary)
	})
}

// Delete removes key and reports whether it existed.
func (k *KV) Delete(key string, boundary int64) (bool, error) {
	var existed bool
	err := k.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec("DELETE FROM kv WHERE session_id=? AND ns=? AND key=?", k.session, k.ns, key)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		existed = n > 0
		if !existed {
			return nil
		}
		return k.journal(tx, key, nil, true, boundary)
	})
	return existed, err
}

// Get returns the encoded value of key.
func (k *KV) Get(key string) (string, bool, error) {
	var v string
	err := k.d.QueryRow("SELECT value_json FROM kv WHERE session_id=? AND ns=? AND key=?", k.session, k.ns, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// Entries is a consistent snapshot of all current entries in byte-wise key order.
func (k *KV) Entries() ([]Entry, error) {
	rows, err := k.d.Query("SELECT key,value_json FROM kv WHERE session_id=? AND ns=? ORDER BY key COLLATE BINARY", k.session, k.ns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Key, &e.JSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EntriesAt reconstructs the entries as they were at a history boundary: every
// mutation recorded at a boundary <= boundary applies.
func (k *KV) EntriesAt(boundary int64) ([]Entry, error) {
	return entriesAt(k.d, k.session, k.ns, boundary)
}

func entriesAt(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, session, ns string, boundary int64) ([]Entry, error) {
	rows, err := q.Query(`SELECT key, value_json FROM journal j
		WHERE session_id=? AND ns=? AND deleted=0 AND boundary<=?
		AND ver = (SELECT MAX(ver) FROM journal WHERE session_id=j.session_id AND ns=j.ns AND key=j.key AND boundary<=?)
		ORDER BY key COLLATE BINARY`, session, ns, boundary, boundary)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Key, &e.JSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
