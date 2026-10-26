package api

import "encoding/json"

// ConfigSettings is the daemon wire form of config.Settings.
type ConfigSettings struct {
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	Effort               string `json:"effort"`
	Tool                 string `json:"tool"`
	View                 string `json:"view"`
	TurnSummary          string `json:"turn_summary"`
	Theme                string `json:"theme"`
	Notify               string `json:"notify"`
	NotifyCommand        string `json:"notify_command"`
	NotifySound          string `json:"notify_sound"`
	NotifyVolume         int    `json:"notify_volume"`
	CompactPercent       int    `json:"compact_percent"`
	CompactKeep          int    `json:"compact_keep"`
	CompactSummaryTokens int    `json:"compact_summary_tokens"`
	ElidePercent         int    `json:"elide_percent"`
	SystemPrompt         string `json:"system_prompt"`
	MaxSpawnDepth        int    `json:"max_spawn_depth"`
	MaxWorkers           int    `json:"max_workers"`
}

type SettingsRequest struct {
	Settings ConfigSettings `json:"settings"`
}
type SettingsResponse struct {
	Settings ConfigSettings `json:"settings"`
}

// SettingsPatch changes only the named settings fields. Expect holds the value
// the client last saw for each field; when the stored value differs, the field
// was changed elsewhere and the patch is rejected with a conflict, never applied
// over the other edit.
type SettingsPatch struct {
	Fields map[string]json.RawMessage `json:"fields"`
	Expect map[string]json.RawMessage `json:"expect,omitempty"`
}
