package app

import "meno/internal/config"

// subagentDefaults picks provider, model and effort for a new subagent. Each
// field falls back on its own: the value passed to agents.spawn, then the
// subagent_* setting, then the value of the calling agent.
func subagentDefaults(s config.Settings, spec map[string]any, provider, model, effort string) (string, string, string) {
	pick := func(key, setting, inherited string) string {
		if _, ok := spec[key]; ok {
			return inherited // the explicit value is applied and checked by the caller
		}
		if setting != "" {
			return setting
		}
		return inherited
	}
	return pick("provider", s.SubagentProvider, provider),
		pick("model", s.SubagentModel, model),
		pick("effort", s.SubagentEffort, effort)
}
