package config

import (
	"database/sql"
	"encoding/json"
	"sort"

	"meno/internal/db"
	"meno/internal/llm"
)

// Store keeps settings, skill state and the model catalog in meno.db. Providers
// and MCP servers live in provider.json and mcp.json, see providers_file.go.
type Store struct {
	P Paths
}

func NewStore(p Paths) *Store { return &Store{P: p} }

// DB returns the shared database handle.
func (s *Store) DB() (*sql.DB, error) { return db.Open(s.P.Home) }

// tx runs fn in a transaction.
func (s *Store) tx(fn func(*sql.Tx) error) error {
	d, err := s.DB()
	if err != nil {
		return err
	}
	return runTx(d, fn)
}

func runTx(d *sql.DB, fn func(*sql.Tx) error) error {
	t, err := d.Begin()
	if err != nil {
		return err
	}
	if err := fn(t); err != nil {
		_ = t.Rollback()
		return err
	}
	return t.Commit()
}

func (s *Store) LoadSettings() (Settings, error) {
	st := DefaultSettings()
	d, err := s.DB()
	if err != nil {
		return st, err
	}
	rows, err := d.Query("SELECT key, value_json FROM settings")
	if err != nil {
		return st, err
	}
	defer rows.Close()
	m := map[string]json.RawMessage{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		m[k] = json.RawMessage(v)
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	if len(m) == 0 {
		return st, nil
	}
	b, _ := json.Marshal(m)
	if err := json.Unmarshal(b, &st); err != nil {
		return DefaultSettings(), err
	}
	return st, nil
}

func (s *Store) SaveSettings(st Settings) error {
	return s.tx(func(t *sql.Tx) error { return putSettings(t, st) })
}

func putSettings(t *sql.Tx, st Settings) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_strings", "js_string_lines", "js_timing", "js_timeout", "js_timeout_sec"} {
		delete(m, key)
		if _, err := t.Exec("DELETE FROM settings WHERE key = ?", key); err != nil {
			return err
		}
	}
	for k, v := range m {
		if _, err := t.Exec(`INSERT INTO settings(key,value_json) VALUES(?,?)
			ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, k, string(v)); err != nil {
			return err
		}
	}
	return nil
}

// LoadCatalog returns the cached model metadata.
func (s *Store) LoadCatalog() (map[string]llm.ModelInfo, error) {
	d, err := s.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.Query("SELECT model, data FROM model_catalog")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]llm.ModelInfo{}
	for rows.Next() {
		var k, v string
		var mi llm.ModelInfo
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(v), &mi) == nil {
			m[k] = mi
		}
	}
	return m, rows.Err()
}

func (s *Store) SaveCatalog(m map[string]llm.ModelInfo) error {
	return s.tx(func(t *sql.Tx) error { return putCatalog(t, m) })
}

func putCatalog(t *sql.Tx, m map[string]llm.ModelInfo) error {
	if _, err := t.Exec("DELETE FROM model_catalog"); err != nil {
		return err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b, err := json.Marshal(m[k])
		if err != nil {
			return err
		}
		if _, err := t.Exec("INSERT INTO model_catalog(model,data) VALUES(?,?)", k, string(b)); err != nil {
			return err
		}
	}
	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
