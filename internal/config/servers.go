package config

import (
	"meno/internal/mcp"
	"meno/internal/store"
)

// legacyPlaceholders are the example servers older versions created on first
// run. They pointed at things that do not exist, so they are dropped.
var legacyPlaceholders = map[string]string{
	"postgres":   "npx -y @mcp/postgres",
	"github":     "gh-mcp-server",
	"sentry":     "http://localhost:9100",
	"playwright": "npx -y @mcp/playwright",
}

// isLegacyPlaceholders reports whether sv is exactly the old example set.
func isLegacyPlaceholders(sv []mcp.Server) bool {
	if len(sv) != len(legacyPlaceholders) {
		return false
	}
	for _, s := range sv {
		target := s.Command
		if s.URL != "" {
			target = s.URL
		}
		if want, ok := legacyPlaceholders[s.Name]; !ok || want != target {
			return false
		}
	}
	return true
}

func (s *Store) LoadServers() ([]mcp.Server, error) {
	var sv []mcp.Server
	err := store.LoadJSON(s.P.File("mcp.json"), &sv)
	if err == nil && isLegacyPlaceholders(sv) {
		sv = nil
		err = store.SaveJSON(s.P.File("mcp.json"), []mcp.Server{})
	}
	return sv, err
}

func (s *Store) SaveServers(sv []mcp.Server) error {
	return store.SaveJSON(s.P.File("mcp.json"), sv)
}
