package engine

import (
	"fmt"
	"os"

	"meno/internal/config"
)

// reloadConfig reads settings, providers and servers again. The config service
// changes them while the daemon runs (a login, /settings, /model), and a run
// must see what the user saved a moment ago. It is cheap: a few small reads.
//
// It returns an error when the settings cannot be read. The last valid snapshot
// stays in place for display and for work that already runs; a caller that is
// about to start a run must stop on the error instead of using stale defaults.
func (e *Engine) reloadConfig() error {
	st := config.NewStore(e.paths)
	settings, err := loadSettings(e.resources, st)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	providers, perr := st.LoadProvidersResolved(os.Getenv)
	if perr != nil {
		// A file broken by hand keeps the last good list.
		e.mu.Lock()
		providers = e.providers
		e.mu.Unlock()
	}
	if e.resources != nil {
		// The config service supplies the current resolved provider list.
		if all := e.providersFromService(); len(all) > 0 {
			providers = all
		}
	}
	servers, serr := st.LoadServers()
	if serr != nil {
		e.mu.Lock()
		servers = e.servers
		e.mu.Unlock()
	}
	catalog, _ := st.LoadCatalog()
	e.mu.Lock()
	e.settings, e.providers, e.servers = settings, providers, servers
	if len(catalog) > 0 {
		e.catalog = catalog
	}
	e.mu.Unlock()
	return nil
}
