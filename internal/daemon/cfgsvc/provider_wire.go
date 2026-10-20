package cfgsvc

import (
	"encoding/json"

	"meno/internal/llm"
	"meno/internal/proto"
)

func providerWire(p llm.Provider) proto.ConfigProvider {
	raw, _ := json.Marshal(p)
	var out proto.ConfigProvider
	_ = json.Unmarshal(raw, &out)
	out.Source = p.Source
	out.HasAPIKey = p.APIKey != ""
	out.APIKey = ""
	out.ClearAPIKey = false
	return out
}
func providerValue(p proto.ConfigProvider) llm.Provider {
	raw, _ := json.Marshal(p)
	var out llm.Provider
	_ = json.Unmarshal(raw, &out)
	out.Source = p.Source
	return out
}
func modelSettingsValue(m map[string]proto.ConfigModelSettings) map[string]llm.ModelSettings {
	raw, _ := json.Marshal(m)
	var out map[string]llm.ModelSettings
	_ = json.Unmarshal(raw, &out)
	return out
}
