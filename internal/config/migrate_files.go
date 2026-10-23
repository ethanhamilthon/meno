package config

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

const (
	migratedKey = "config_files"
	backupTag   = ".bak-0.6.7"
)

// MigrateToFiles moves the providers and MCP servers that 0.6.7 and earlier
// kept in meno.db into provider.json and mcp.json. It runs once, at daemon start,
// before any service reads the files.
//
// The database entries come first, in their old order; entries the file already
// had follow, unless a database entry has the same name (the database wins, as it
// did before). A file that changes is copied to NAME.bak-0.6.7 first. The tables
// stay in the database, unread. The flag is set only after both files are
// written, so a failure is retried at the next start.
func MigrateToFiles(p Paths) error {
	listMu.Lock()
	defer listMu.Unlock()
	if _, err := os.Stat(filepath.Join(p.Home, db.File)); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	d, err := db.Open(p.Home)
	if err != nil {
		return err
	}
	var done string
	err = d.QueryRow("SELECT value FROM meta WHERE key = ?", migratedKey).Scan(&done)
	if err == nil && done != "" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	oldProviders, err := tableProviders(d)
	if err != nil {
		return err
	}
	oldServers, err := tableServers(d)
	if err != nil {
		return err
	}
	if err := migrateProviders(p, oldProviders); err != nil {
		return err
	}
	if err := migrateServers(p, oldServers); err != nil {
		return err
	}
	_, err = d.Exec("INSERT OR REPLACE INTO meta(key,value) VALUES(?,?)", migratedKey, "1")
	return err
}

// backup copies the file to NAME.bak-0.6.7 once; an existing backup stays.
func backup(path string) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dst := path + backupTag
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return os.WriteFile(dst, b, 0o600)
}

func migrateProviders(p Paths, old []llm.Provider) error {
	if len(old) == 0 {
		return nil
	}
	path := p.ProviderFile()
	raw, err := readForWrite(path)
	if err != nil {
		return fmt.Errorf("migrating providers: %w", err)
	}
	have, bad, _ := decodeProviders(raw, ProviderFileName)
	names := map[string]bool{}
	for _, o := range old {
		names[o.Name] = true
	}
	merged := append([]llm.Provider(nil), old...)
	for _, f := range have {
		if !names[f.Name] {
			merged = append(merged, f)
		}
	}
	if err := backup(path); err != nil {
		return err
	}
	return (&Store{P: p}).saveProviderEntries(merged, bad)
}

func migrateServers(p Paths, old []mcp.Server) error {
	if len(old) == 0 {
		return nil
	}
	path := p.MCPFile()
	raw, err := readForWrite(path)
	if err != nil {
		return fmt.Errorf("migrating MCP servers: %w", err)
	}
	have, bad, _ := decodeServers(raw, MCPFileName)
	names := map[string]bool{}
	for _, o := range old {
		names[o.Name] = true
	}
	merged := append([]mcp.Server(nil), old...)
	for _, f := range have {
		if !names[f.Name] {
			merged = append(merged, f)
		}
	}
	if err := backup(path); err != nil {
		return err
	}
	return (&Store{P: p}).saveServerEntries(merged, bad)
}

func tableProviders(d *sql.DB) ([]llm.Provider, error) {
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

func tableServers(d *sql.DB) ([]mcp.Server, error) {
	rows, err := d.Query("SELECT name, transport, command, url, enabled, env_json, headers_json, args_json, cwd FROM mcp_servers ORDER BY pos")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sv []mcp.Server
	for rows.Next() {
		var x mcp.Server
		var en int
		var env, hdr, args string
		if err := rows.Scan(&x.Name, &x.Transport, &x.Command, &x.URL, &en, &env, &hdr, &args, &x.Cwd); err != nil {
			return nil, err
		}
		x.Enabled = en != 0
		_ = json.Unmarshal([]byte(env), &x.Env)
		_ = json.Unmarshal([]byte(hdr), &x.Headers)
		_ = json.Unmarshal([]byte(args), &x.Args)
		if len(x.Env) == 0 {
			x.Env = nil
		}
		if len(x.Headers) == 0 {
			x.Headers = nil
		}
		if len(x.Args) == 0 {
			x.Args = nil
		}
		sv = append(sv, x)
	}
	return sv, rows.Err()
}
