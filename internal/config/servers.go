package config

import (
	"meno/internal/mcp"
	"meno/internal/store"
)

func (s *Store) LoadServers() ([]mcp.Server, error) {
	var sv []mcp.Server
	err := store.LoadJSON(s.P.File("mcp.json"), &sv)
	if sv == nil && err == nil {
		sv = mcp.DefaultServers()
	}
	return sv, err
}

func (s *Store) SaveServers(sv []mcp.Server) error {
	return store.SaveJSON(s.P.File("mcp.json"), sv)
}
