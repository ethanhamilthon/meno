package config

import (
	"encoding/json"
	"fmt"

	"meno/internal/mcp"
	"meno/internal/store"
)

// decodeServers splits the entries of mcp.json like decodeProviders does. A
// server without "enabled" is enabled and without "transport" is stdio.
func decodeServers(raw []json.RawMessage, file string) (ok []mcp.Server, bad []json.RawMessage, warns []string) {
	seen := map[string]bool{}
	for i, r := range raw {
		var f struct {
			mcp.Server
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal(r, &f); err != nil {
			warns = append(warns, fmt.Sprintf("%s: entry %d: %v", file, i+1, err))
			bad = append(bad, r)
			continue
		}
		s := f.Server
		s.Enabled = f.Enabled == nil || *f.Enabled
		if s.Transport == "" {
			s.Transport = mcp.TransportStdio
		}
		if err := mcp.ValidateServer(s); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %s: %v", file, label(s.Name, i), err))
			bad = append(bad, r)
			continue
		}
		if seen[s.Name] {
			warns = append(warns, fmt.Sprintf("%s: %s: duplicate name, the first one is used", file, s.Name))
			bad = append(bad, r)
			continue
		}
		seen[s.Name] = true
		ok = append(ok, s)
	}
	return ok, bad, warns
}

// LoadServersFile reads mcp.json with its warnings. An invalid entry is skipped.
func (s *Store) LoadServersFile() ([]mcp.Server, []string, error) {
	listMu.Lock()
	defer listMu.Unlock()
	raw, err := readFile(s.P.MCPFile())
	if err != nil {
		return nil, nil, err
	}
	sv, _, warns := decodeServers(raw, MCPFileName)
	return sv, warns, nil
}

// LoadServers returns the MCP servers in mcp.json.
func (s *Store) LoadServers() ([]mcp.Server, error) {
	sv, _, err := s.LoadServersFile()
	return sv, err
}

// SaveServers writes the servers to mcp.json with the same care as
// SaveProviders: unreadable entries stay, a broken file is not overwritten.
func (s *Store) SaveServers(sv []mcp.Server) error {
	listMu.Lock()
	defer listMu.Unlock()
	raw, err := readForWrite(s.P.MCPFile())
	if err != nil {
		return err
	}
	_, bad, _ := decodeServers(raw, MCPFileName)
	return s.saveServerEntries(sv, bad)
}

func (s *Store) saveServerEntries(sv []mcp.Server, bad []json.RawMessage) error {
	out := []json.RawMessage{}
	for _, x := range sv {
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		out = append(out, b)
	}
	return store.SaveJSON(s.P.MCPFile(), append(out, bad...))
}
