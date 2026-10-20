package engine

import "meno/internal/config"

// reloadConfig reads settings, providers and servers again. The config service
// changes them while the daemon runs (a login, :settings, :model), and a run
// must see what the user saved a moment ago. It is cheap: a few small reads.
func (e *Engine) reloadConfig() {
	st := config.NewStore(e.paths)
	settings, err := st.LoadSettings()
	if err != nil {
		return
	}
	providers, err := st.LoadProviders()
	if err != nil {
		return
	}
	if e.resources != nil {
		// The config service knows the full list: saved providers, provider.json
		// and the one derived from the OAuth accounts.
		if all := e.providersFromService(); len(all) > 0 {
			providers = all
		}
	}
	servers, _ := st.LoadServers()
	serverFiles, _ := config.LoadMCPFile(e.paths.MCPFile())
	servers, _ = config.MergeServers(servers, serverFiles)
	catalog, _ := st.LoadCatalog()
	e.mu.Lock()
	e.settings, e.providers, e.servers = settings, providers, servers
	if len(catalog) > 0 {
		e.catalog = catalog
	}
	e.mu.Unlock()
}
