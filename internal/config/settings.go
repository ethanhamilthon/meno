package config

import (
	"encoding/json"
	"strconv"
	"strings"
)

// DefaultMaxSpawn is how many subagents one request may start before the run
// refuses more; the limit is subagent_max_spawn in the settings.
const DefaultMaxSpawn = 50

type Settings struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	JSCode   string `json:"js_code"`
	JSLines  int    `json:"js_lines"`
	JSOutput string `json:"js_output"`
	// JSStrings is off or lines: with lines, string literals in tool-call source
	// show only their first JSStringLines lines of content (0 = just a marker).
	JSStrings     string `json:"js_strings"`
	JSStringLines int    `json:"js_string_lines"`
	// JSOutLines is the row limit of js output in "lines" mode.
	JSOutLines int `json:"js_output_lines"`
	// Thinking is all/lines/off like js code; ThinkLines is its row limit.
	// JSTiming is how long each js call is shown to have taken: all calls,
	// only the slow ones, or never. TurnSummary is the line under the answer
	// that sums up the work (on/off).
	JSTiming    string `json:"js_timing"`
	TurnSummary string `json:"turn_summary"`
	Thinking    string `json:"thinking"`
	ThinkLines  int    `json:"thinking_lines"`
	// JSTimeout is sec (JSTimeoutSec seconds), off (no limit) or model (the model
	// passes a timeout with each call).
	JSTimeout    string `json:"js_timeout"`
	JSTimeoutSec int    `json:"js_timeout_sec"`
	// Theme is the name of a ui theme; the app falls back to the default for
	// names it does not know.
	Theme string `json:"theme"`
	// Notify is when to signal that the user is needed: off, question (the agent
	// asks with askUser) or done (also when a turn ends, with success or error).
	Notify string `json:"notify"`
	// NotifyCommand is the shell command that makes the signal. Empty: the
	// default of the platform (a sound on macOS, else the terminal bell);
	// "bell": the terminal bell.
	NotifyCommand        string `json:"notify_command"`
	NotifySound          string `json:"notify_sound"`
	NotifyVolume         int    `json:"notify_volume"`
	EffortRouter         string `json:"effort_router,omitempty"`
	EffortRouterModel    string `json:"effort_router_model,omitempty"`
	EffortRouterProvider string `json:"effort_router_provider,omitempty"`
	// WebSearch picks the fallback search backend of websearch(): auto,
	// brave, tavily, duckduckgo or off. WebSearchKey is the Brave or Tavily key.
	WebSearch    string `json:"websearch"`
	WebSearchKey string `json:"websearch_key"`
	// Subagent defaults when agents.spawn omits provider, model, or effort.
	SubagentProvider string `json:"subagent_provider,omitempty"`
	SubagentModel    string `json:"subagent_model,omitempty"`
	SubagentEffort   string `json:"subagent_effort,omitempty"`
	// SubagentMaxSpawn is how many subagents one request may start. Zero or less
	// means the default (see DefaultMaxSpawn).
	SubagentMaxSpawn int `json:"subagent_max_spawn,omitempty"`
	// CompactPercent is the share of the context window at which meno compacts
	// before a request; CompactKeep the recent messages that always stay;
	// CompactSummaryTokens the target size of the summary.
	CompactPercent       int `json:"compact_percent"`
	CompactKeep          int `json:"compact_keep"`
	CompactSummaryTokens int `json:"compact_summary_tokens"`
	// ImagesProtocol is auto, off, kitty, sixel or iterm: which terminal
	// graphics protocol draws images in the timeline.
	ImagesProtocol string `json:"images_protocol"`
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
	defaults := DefaultSettings()
	// Values from before js code/output shared the same all/lines/off choices.
	if st.JSCode == "full" {
		st.JSCode = "all"
	}
	switch st.JSOutput {
	case "on":
		st.JSOutput = "all"
	}
	if st.Thinking == "on" {
		st.Thinking = "all"
	}
	// Timeouts used to be 5s / 30s / none.
	if n, err := strconv.Atoi(strings.TrimSuffix(st.JSTimeout, "s")); err == nil && strings.HasSuffix(st.JSTimeout, "s") && n > 0 {
		st.JSTimeout, st.JSTimeoutSec = "sec", min(n, MaxTimeoutSec)
	} else if st.JSTimeout == "none" {
		st.JSTimeout = "off"
	}
	for key := range choices {
		if err := st.Set(key, st.Get(key)); err != nil {
			st.setRaw(key, defaults.Get(key))
		}
	}
	if st.Theme == "" {
		st.Theme = defaults.Theme
	}
	if st.JSLines < 1 || st.JSLines > MaxLines {
		st.JSLines = defaults.JSLines
	}
	if st.JSStringLines < 0 || st.JSStringLines > MaxLines {
		st.JSStringLines = defaults.JSStringLines
	}
	if st.JSOutLines < 1 || st.JSOutLines > MaxLines {
		st.JSOutLines = defaults.JSOutLines
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
	if st.JSTimeoutSec < 1 || st.JSTimeoutSec > MaxTimeoutSec {
		st.JSTimeoutSec = defaults.JSTimeoutSec
	}
	if st.SubagentMaxSpawn < 1 {
		st.SubagentMaxSpawn = DefaultMaxSpawn
	}
	*s = st
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Effort:         "high",
		EffortRouter:   "off",
		JSCode:         "all",
		JSLines:        5,
		JSOutput:       "all",
		JSOutLines:     5,
		JSStrings:      "off",
		JSStringLines:  3,
		Thinking:       "all",
		JSTiming:       "all",
		TurnSummary:    "on",
		ThinkLines:     5,
		Theme:          "meno",
		JSTimeout:      "sec",
		JSTimeoutSec:   30,
		CompactPercent: 80, CompactKeep: 8, CompactSummaryTokens: 4000,
		Notify:           "off",
		NotifySound:      "default",
		NotifyVolume:     100,
		WebSearch:        "auto",
		SubagentMaxSpawn: DefaultMaxSpawn,
	}
}
