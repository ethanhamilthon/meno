package config

import (
	"database/sql"
	"encoding/json"
	"sort"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

// Store keeps settings, providers, MCP servers, skill state and the model
// catalog in meno.db. Files such as provider.json only add to it, see overlay.go.
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
	for k, v := range m {
		if _, err := t.Exec(`INSERT INTO settings(key,value_json) VALUES(?,?)
			ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, k, string(v)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LoadProviders() ([]llm.Provider, error) {
	d, err := s.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.Query("SELECT name, base_url, api_key, protocol, run, models_json, disabled FROM providers ORDER BY pos")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []llm.Provider
	for rows.Next() {
		var p llm.Provider
		var models string
		var off int
		if err := rows.Scan(&p.Name, &p.BaseURL, &p.APIKey, &p.Protocol, &p.Run, &models, &off); err != nil {
			return nil, err
		}
		p.Disabled = off != 0
		// A damaged value costs the model settings of this provider, not the provider.
		if err := json.Unmarshal([]byte(models), &p.Models); err != nil || len(p.Models) == 0 {
			p.Models = nil
		}
		ps = append(ps, p)
	}
	return ps, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) SaveProviders(ps []llm.Provider) error {
	return s.tx(func(t *sql.Tx) error { return putProviders(t, ps) })
}

func putProviders(t *sql.Tx, ps []llm.Provider) error {
	if _, err := t.Exec("DELETE FROM providers"); err != nil {
		return err
	}
	for i, p := range ps {
		models := []byte("{}")
		if len(p.Models) > 0 {
			models, _ = json.Marshal(p.Models)
		}
		if _, err := t.Exec("INSERT INTO providers(pos,name,base_url,api_key,protocol,run,models_json,disabled) VALUES(?,?,?,?,?,?,?,?)",
			i, p.Name, p.BaseURL, p.APIKey, p.Protocol, p.Run, string(models), boolInt(p.Disabled)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LoadServers() ([]mcp.Server, error) {
	d, err := s.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.Query("SELECT name, transport, command, url, enabled, env_json, headers_json FROM mcp_servers ORDER BY pos")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sv []mcp.Server
	for rows.Next() {
		var x mcp.Server
		var en int
		var env, hdr string
		if err := rows.Scan(&x.Name, &x.Transport, &x.Command, &x.URL, &en, &env, &hdr); err != nil {
			return nil, err
		}
		x.Enabled = en != 0
		_ = json.Unmarshal([]byte(env), &x.Env)
		_ = json.Unmarshal([]byte(hdr), &x.Headers)
		if len(x.Env) == 0 {
			x.Env = nil
		}
		if len(x.Headers) == 0 {
			x.Headers = nil
		}
		sv = append(sv, x)
	}
	return sv, rows.Err()
}

func (s *Store) SaveServers(sv []mcp.Server) error {
	return s.tx(func(t *sql.Tx) error { return putServers(t, sv) })
}

func putServers(t *sql.Tx, sv []mcp.Server) error {
	if _, err := t.Exec("DELETE FROM mcp_servers"); err != nil {
		return err
	}
	for i, x := range sv {
		env, _ := json.Marshal(x.Env)
		hdr, _ := json.Marshal(x.Headers)
		if x.Env == nil {
			env = []byte("{}")
		}
		if x.Headers == nil {
			hdr = []byte("{}")
		}
		if _, err := t.Exec("INSERT INTO mcp_servers(pos,name,transport,command,url,enabled,env_json,headers_json) VALUES(?,?,?,?,?,?,?,?)",
			i, x.Name, x.Transport, x.Command, x.URL, b2i(x.Enabled), string(env), string(hdr)); err != nil {
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
