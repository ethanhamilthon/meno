package app

import (
	"path/filepath"

	"meno/internal/config"
)

// agentsPaths returns the global and the project AGENTS.md locations.
func agentsPaths(p config.Paths) (global, project string) {
	return filepath.Join(p.Home, "AGENTS.md"), filepath.Join(p.Cwd, "AGENTS.md")
}
