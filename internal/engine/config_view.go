package engine

import (
	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
)

// The config is replaced as a whole by reloadConfig, so a reader takes a copy
// of the part it needs under the lock instead of looking at the field.

func (e *Engine) cfg() config.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

func (e *Engine) providerList() []llm.Provider {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]llm.Provider(nil), e.providers...)
}

func (e *Engine) serverList() []mcp.Server {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]mcp.Server(nil), e.servers...)
}

func (e *Engine) catalogView() map[string]llm.ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.catalog
}

// elideOptions turns the settings into the first stage of context saving. An
// elide threshold at or above the compaction threshold never acts (compaction
// runs first), which is how it is switched off.
func elideOptions(st config.Settings) agent.ElideOptions {
	if st.ElidePercent >= st.CompactPercent {
		return agent.ElideOptions{}
	}
	return agent.ElideOptions{Percent: st.ElidePercent}
}
