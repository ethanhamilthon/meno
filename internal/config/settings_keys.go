package config

import (
	"fmt"
	"strconv"
	"strings"

	"meno/internal/notify"
)

var choices = map[string][]string{
	"js_code":         {"all", "lines", "off"},
	"js_output":       {"all", "lines", "off"},
	"js_strings":      {"off", "lines"},
	"images":          {"auto", "off", "kitty", "sixel", "iterm"},
	"thinking":        {"all", "lines", "off"},
	"js_timeout":      {"sec", "off", "model"},
	"js_timing":       {"all", "slow", "off"},
	"turn_summary":    {"on", "off"},
	"effort":          {"low", "medium", "high", "max"},
	"effort_router":   {"off", "auto"},
	"notify":          {"off", "question", "done"},
	"websearch":       {"auto", "brave", "tavily", "duckduckgo", "off"},
	"notify_volume":   {"20", "40", "60", "80", "100"},
	"subagent_effort": {"", "low", "medium", "high", "xhigh"},
	// One choice: the row only carries the number (the threshold in percent).
	"compact_percent": {"percent"},
	"notify_sound":    append([]string{"default", "bell"}, notify.Names()...),
}

func SettingKeys() []string {
	return append([]string{"theme", "images", "thinking", "turn_summary", "notify", "notify_sound", "notify_volume", "notify_command", "websearch", "websearch_key", "subagent_provider", "subagent_model", "subagent_effort", "effort_router", "effort_router_model", "effort_router_provider", "compact_percent"}, bashKeys...)
}

func Choices(key string) []string {
	if c, ok := bashChoices[key]; ok {
		return c
	}
	return choices[key]
}

func (s *Settings) Get(key string) string {
	if s.Tool == "bash" && strings.HasPrefix(key, "js_") && key != "js_timeout" {
		key = canonicalSettingKey(key)
	}
	if value, ok := getBash(s, key); ok {
		return value
	}
	switch key {
	case "js_code":
		return s.JSCode
	case "js_output":
		return s.JSOutput
	case "js_strings":
		return s.JSStrings
	case "images":
		return s.ImagesProtocol
	case "thinking":
		return s.Thinking
	case "js_timeout":
		return s.JSTimeout
	case "js_timing":
		return s.JSTiming
	case "turn_summary":
		return s.TurnSummary
	case "effort":
		return s.Effort
	case "effort_router":
		return s.EffortRouter
	case "effort_router_model":
		return s.EffortRouterModel
	case "effort_router_provider":
		return s.EffortRouterProvider
	case "theme":
		return s.Theme
	case "notify_sound":
		return s.NotifySound
	case "notify_volume":
		return strconv.Itoa(s.NotifyVolume)
	case "notify":
		return s.Notify
	case "websearch":
		return s.WebSearch
	case "websearch_key":
		return s.WebSearchKey
	case "notify_command":
		return s.NotifyCommand
	case "subagent_provider":
		return s.SubagentProvider
	case "subagent_model":
		return s.SubagentModel
	case "subagent_effort":
		return s.SubagentEffort
	case "compact_percent":
		return "percent"
	}
	return ""
}

func (s *Settings) Set(key, val string) error {
	if handled, err := setBashSetting(s, key, val); handled {
		return err
	}
	switch key {
	case "theme":
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("invalid value %q for %s", val, key)
		}
		s.Theme = val
		return nil
	case "websearch_key":
		s.WebSearchKey = strings.TrimSpace(val)
		return nil
	case "effort_router_model":
		s.EffortRouterModel = strings.TrimSpace(val)
		return nil
	case "effort_router_provider":
		s.EffortRouterProvider = strings.TrimSpace(val)
		return nil
	case "notify_command":
		s.NotifyCommand = strings.TrimSpace(val)
		return nil
	case "subagent_provider", "subagent_model":
		if val == "inherit" {
			val = ""
		}
		s.setRaw(key, val)
		return nil
	case "subagent_effort":
		if val == "inherit" {
			val = ""
		}
	case "notify_volume":
		val = strings.TrimSuffix(val, "%")
	}
	opts, ok := choices[key]
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	for _, o := range opts {
		if o == val {
			s.setRaw(key, val)
			return nil
		}
	}
	return fmt.Errorf("invalid value %q for %s", val, key)
}

func (s *Settings) setRaw(key, val string) {
	switch key {
	case "theme":
		s.Theme = val
	case "js_code":
		s.JSCode = val
	case "js_output":
		s.JSOutput = val
	case "js_strings":
		s.JSStrings = val
	case "images":
		s.ImagesProtocol = val
	case "thinking":
		s.Thinking = val
	case "js_timeout":
		s.JSTimeout = val
	case "js_timing":
		s.JSTiming = val
	case "turn_summary":
		s.TurnSummary = val
	case "effort":
		s.Effort = val
	case "effort_router":
		s.EffortRouter = val
	case "effort_router_model":
		s.EffortRouterModel = val
	case "effort_router_provider":
		s.EffortRouterProvider = val
	case "notify_sound":
		s.NotifySound = val
	case "notify_volume":
		if n, err := strconv.Atoi(val); err == nil {
			s.NotifyVolume = n
		}
	case "notify":
		s.Notify = val
	case "websearch":
		s.WebSearch = val
	case "websearch_key":
		s.WebSearchKey = val
	case "subagent_provider":
		s.SubagentProvider = val
	case "subagent_model":
		s.SubagentModel = val
	case "subagent_effort":
		s.SubagentEffort = val
	}
}

// TextSetting reports whether a setting is free text instead of a choice.
func TextSetting(key string) bool {
	return key == "websearch_key" || key == "notify_command" || key == "effort_router_model" || key == "effort_router_provider"
}

func (s *Settings) Cycle(key string, step int) string {
	if _, ok := bashChoices[key]; ok {
		opts := bashChoices[key]
		cur := s.Get(key)
		idx := 0
		for i, v := range opts {
			if v == cur {
				idx = i
			}
		}
		idx = ((idx+step)%len(opts) + len(opts)) % len(opts)
		setBash(s, key, opts[idx])
		return opts[idx]
	}
	opts := choices[key]
	cur := s.Get(key)
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
		}
	}
	n := len(opts)
	idx = ((idx+step)%n + n) % n
	s.setRaw(key, opts[idx])
	return opts[idx]
}
