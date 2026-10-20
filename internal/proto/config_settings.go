package proto

// ConfigSettings is the daemon wire form of config.Settings.
type ConfigSettings struct {
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	Effort               string `json:"effort"`
	JSCode               string `json:"js_code"`
	JSLines              int    `json:"js_lines"`
	JSOutput             string `json:"js_output"`
	JSStrings            string `json:"js_strings"`
	JSStringLines        int    `json:"js_string_lines"`
	JSOutLines           int    `json:"js_output_lines"`
	JSTiming             string `json:"js_timing"`
	TurnSummary          string `json:"turn_summary"`
	Thinking             string `json:"thinking"`
	ThinkLines           int    `json:"thinking_lines"`
	JSTimeout            string `json:"js_timeout"`
	JSTimeoutSec         int    `json:"js_timeout_sec"`
	Theme                string `json:"theme"`
	Notify               string `json:"notify"`
	NotifyCommand        string `json:"notify_command"`
	NotifySound          string `json:"notify_sound"`
	NotifyVolume         int    `json:"notify_volume"`
	EffortRouter         string `json:"effort_router,omitempty"`
	EffortRouterModel    string `json:"effort_router_model,omitempty"`
	EffortRouterProvider string `json:"effort_router_provider,omitempty"`
	WebSearch            string `json:"websearch"`
	WebSearchKey         string `json:"websearch_key"`
	SubagentProvider     string `json:"subagent_provider,omitempty"`
	SubagentModel        string `json:"subagent_model,omitempty"`
	SubagentEffort       string `json:"subagent_effort,omitempty"`
	SubagentMaxSpawn     int    `json:"subagent_max_spawn,omitempty"`
	CompactPercent       int    `json:"compact_percent"`
	CompactKeep          int    `json:"compact_keep"`
	CompactSummaryTokens int    `json:"compact_summary_tokens"`
	ImagesProtocol       string `json:"images_protocol"`
}

type SettingsRequest struct {
	Settings ConfigSettings `json:"settings"`
}
type SettingsResponse struct {
	Settings ConfigSettings `json:"settings"`
}
