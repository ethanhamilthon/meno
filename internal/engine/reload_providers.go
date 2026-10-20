package engine

import (
	"meno/internal/config"
	"meno/internal/llm"
)

// providerSource is a config service that can hand over the full provider list
// (with API keys) to the engine inside the daemon process.
type providerSource interface{ Providers() []llm.Provider }

// settingsSource is a config service that can hand over its current settings
// snapshot. The engine and the settings route then read one source.
type settingsSource interface {
	Settings() (config.Settings, error)
}

// providersFromService asks the config service for every provider it knows.
func (e *Engine) providersFromService() []llm.Provider {
	if src, ok := e.resources.(providerSource); ok {
		return src.Providers()
	}
	return nil
}

// loadSettings returns the current settings from the config service when there
// is one, and otherwise from the shared file-first loader.
func loadSettings(res ResourceHandler, st *config.Store) (config.Settings, error) {
	if src, ok := res.(settingsSource); ok {
		return src.Settings()
	}
	return st.LoadCurrentSettings()
}
