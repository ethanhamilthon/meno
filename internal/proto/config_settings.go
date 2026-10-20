package proto

// ConfigSettings is the daemon wire form of config.Settings.
type ConfigSettings struct {
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	Effort               string `json:"effort"`
	Tool                 string `json:"tool"`
	BashCode             string `json:"bash_code"`
	BashLines            int    `json:"bash_lines"`
	BashOutput           string `json:"bash_output"`
	BashOutputLines      int    `json:"bash_output_lines"`
	BashStrings          string `json:"bash_strings"`
	BashStringLines      int    `json:"bash_string_lines"`
	BashTiming           string `json:"bash_timing"`
	BashTimeout          string `json:"bash_timeout"`
	TurnSummary          string `json:"turn_summary"`
	Thinking             string `json:"thinking"`
	ThinkLines           int    `json:"thinking_lines"`
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
