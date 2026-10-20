package engine

import "meno/internal/llm"

// providerSource is a config service that can hand over the full provider list
// (with API keys) to the engine inside the daemon process.
type providerSource interface{ Providers() []llm.Provider }

// providersFromService asks the config service for every provider it knows.
func (e *Engine) providersFromService() []llm.Provider {
	if src, ok := e.resources.(providerSource); ok {
		return src.Providers()
	}
	return nil
}
