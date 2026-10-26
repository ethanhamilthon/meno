package config

import (
	"encoding/json"
)

type Settings struct {
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	Effort            string `json:"effort"`
	Tool              string `json:"tool"`
	BashCode          string `json:"bash_code"`
	BashLines         int    `json:"bash_lines"`
	BashOutput        string `json:"bash_output"`
	BashOutputLines   int    `json:"bash_output_lines"`
	BashTiming        string `json:"bash_timing"`
	DaemonIdleExitMin int    `json:"daemon_idle_exit_min"`
	// Thinking is all/lines/off; ThinkLines is its row limit.
	TurnSummary string `json:"turn_summary"`
	Thinking    string `json:"thinking"`
	ThinkLines  int    `json:"thinking_lines"`
	// Theme is the name of a ui theme; the app falls back to the default for
	// names it does not know.
	Theme string `json:"theme"`
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
	// ImagesProtocol is auto, off, kitty, sixel or iterm: which terminal
	// graphics protocol draws images in the timeline.
	ImagesProtocol string `json:"images_protocol"`
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
	if _, hasBash := raw["bash_code"]; !hasBash {
		st.BashCode = defaults.BashCode
	}
	if _, hasBash := raw["bash_lines"]; !hasBash {
		st.BashLines = defaults.BashLines
	}
	if _, hasBash := raw["bash_output"]; !hasBash {
		st.BashOutput = defaults.BashOutput
	}
	if _, hasBash := raw["bash_output_lines"]; !hasBash {
		st.BashOutputLines = defaults.BashOutputLines
	}
	if _, hasBash := raw["bash_timing"]; !hasBash {
		st.BashTiming = defaults.BashTiming
	}
	if _, hasBash := raw["daemon_idle_exit_min"]; !hasBash {
		st.DaemonIdleExitMin = defaults.DaemonIdleExitMin
	}
	applyJSAliases(raw, &st)
	if st.BashCode == "full" {
		st.BashCode = "all"
	}
	switch st.BashOutput {
	case "on":
		st.BashOutput = "all"
	}
	if st.Thinking == "on" {
		st.Thinking = "all"
	}
	for key := range choices {
		if err := st.Set(key, st.Get(key)); err != nil {
			st.setRaw(key, defaults.Get(key))
		}
	}
	for key, value := range map[string]string{"bash_code": st.BashCode, "bash_output": st.BashOutput, "bash_timing": st.BashTiming} {
		if !contains(bashChoices[key], value) {
			setBash(&st, key, defaults.Get(key))
		}
	}
	if st.Theme == "" {
		st.Theme = defaults.Theme
	}
	if st.ThinkLines < 1 || st.ThinkLines > MaxLines {
		st.ThinkLines = defaults.ThinkLines
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
	normalizeBash(&st, defaults)
	if st.Tool != "js" && st.Tool != "bash" {
		st.Tool = defaults.Tool
	}
	*s = st
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Tool: "bash", BashCode: "all", BashLines: 5, BashOutput: "all", BashOutputLines: 5,
		BashTiming: "all", DaemonIdleExitMin: 30,
		Effort:         "high",
		Thinking:       "all",
		TurnSummary:    "on",
		ThinkLines:     5,
		Theme:          "default",
		CompactPercent: 80, CompactKeep: 8, CompactSummaryTokens: 4000, ElidePercent: 50,
		MaxSpawnDepth: 2, MaxWorkers: 8,
		Notify:       "off",
		NotifySound:  "default",
		NotifyVolume: 100,
	}
}
