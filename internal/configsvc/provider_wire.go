package configsvc

import (
	"encoding/json"

	"meno/internal/api"
	"meno/internal/llm"
)

func providerWire(p llm.Provider) api.ConfigProvider {
	raw, _ := json.Marshal(p)
	var out api.ConfigProvider
	_ = json.Unmarshal(raw, &out)
	out.Source = p.Source
	out.HasAPIKey = p.APIKey != ""
	out.APIKey = ""
	out.ClearAPIKey = false
	return out
}
func providerValue(p api.ConfigProvider) llm.Provider {
	raw, _ := json.Marshal(p)
	var out llm.Provider
	_ = json.Unmarshal(raw, &out)
	out.Source = p.Source
	return out
}
func modelSettingsValue(m map[string]api.ConfigModelSettings) map[string]llm.ModelSettings {
	raw, _ := json.Marshal(m)
	var out map[string]llm.ModelSettings
	_ = json.Unmarshal(raw, &out)
	return out
}
