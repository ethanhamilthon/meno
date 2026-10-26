package config

import (
	"encoding/json"
)

type Settings struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	Tool     string `json:"tool"`
	// View is how much of the work the timeline shows: full, bash_only or compact.
	View        string `json:"view"`
	TurnSummary string `json:"turn_summary"`
	// Notify is when to signal that the user is needed: off, question (the agent
	// asks with askUser) or done (also when a turn ends, with success or error).
	Notify string `json:"notify"`
	// NotifyCommand is the shell command that makes the signal. Empty: the
	// default of the platform (a sound on macOS, else the terminal bell);
	// "bell": the terminal bell.
	NotifyCommand string `json:"notify_command"`
	NotifySound   string `json:"notify_sound"`
	NotifyVolume  int    `json:"notify_volume"`
	// CompactPercent is the share of the context window at which meno compacts
	// before a request; CompactKeep the recent messages that always stay;
	// CompactSummaryTokens the target size of the summary.
	CompactPercent       int `json:"compact_percent"`
	CompactKeep          int `json:"compact_keep"`
	CompactSummaryTokens int `json:"compact_summary_tokens"`
	// ElidePercent is the share of the context window at which meno first cuts the
	// bulky output of old tool calls, before it asks the model for a summary.
	// 10 to 100; at or above CompactPercent it never acts, which switches it off.
	ElidePercent int `json:"elide_percent"`
	// SystemPrompt names a file in system-prompts/ whose text replaces the
	// identity line of the system prompt in new sessions. Empty is the built-in line.
	SystemPrompt string `json:"system_prompt"`
	// MaxSpawnDepth bounds how deep sessions started by `meno send` can nest;
	// MaxWorkers bounds how many sessions one session can have running.
	MaxSpawnDepth int `json:"max_spawn_depth"`
	MaxWorkers    int `json:"max_workers"`
}

// UnmarshalJSON restores safe defaults for obsolete enum values and invalid
// ranges. Decode into a copy so malformed JSON cannot partially alter settings.
func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	next := plain(DefaultSettings())
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	// The bell for questions used to be notify_question on/off.
	var legacy struct {
		Notify   *string `json:"notify"`
		Question string  `json:"notify_question"`
	}
	if json.Unmarshal(data, &legacy) == nil && legacy.Notify == nil && legacy.Question == "on" {
		next.Notify = "question"
	}
	st := Settings(next)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	defaults := DefaultSettings()
	if _, hasTool := raw["tool"]; !hasTool && hasLegacyJSSetting(raw) {
		st.Tool = "js"
	}
	for key := range choices {
		if err := st.Set(key, st.Get(key)); err != nil {
			st.setRaw(key, defaults.Get(key))
		}
	}
	if st.CompactPercent < MinCompactPercent || st.CompactPercent > MaxCompactPercent {
		st.CompactPercent = defaults.CompactPercent
	}
	if st.CompactKeep < 2 || st.CompactKeep > 200 {
		st.CompactKeep = defaults.CompactKeep
	}
	if st.CompactSummaryTokens < 500 || st.CompactSummaryTokens > 30000 {
		st.CompactSummaryTokens = defaults.CompactSummaryTokens
	}
	if st.ElidePercent < 10 || st.ElidePercent > 100 {
		st.ElidePercent = defaults.ElidePercent
	}
	if st.SystemPrompt == DefaultSystemPrompt || ValidSystemPromptName(st.SystemPrompt) != nil {
		st.SystemPrompt = ""
	}
	if st.MaxSpawnDepth < 1 || st.MaxSpawnDepth > 10 {
		st.MaxSpawnDepth = defaults.MaxSpawnDepth
	}
	if st.MaxWorkers < 1 || st.MaxWorkers > 64 {
		st.MaxWorkers = defaults.MaxWorkers
	}
	if st.Tool != "js" && st.Tool != "bash" {
		st.Tool = defaults.Tool
	}
	*s = st
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Tool: "bash", View: "compact",
		Effort:         "high",
		TurnSummary:    "on",
		CompactPercent: 80, CompactKeep: 8, CompactSummaryTokens: 4000, ElidePercent: 50,
		MaxSpawnDepth: 2, MaxWorkers: 8,
		Notify:       "off",
		NotifySound:  "default",
		NotifyVolume: 100,
	}
}
