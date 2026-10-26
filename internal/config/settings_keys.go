package config

import (
	"fmt"
	"strconv"
	"strings"

	"meno/internal/notify"
)

var choices = map[string][]string{
	"view":          {"full", "bash_only", "compact"},
	"turn_summary":  {"on", "off"},
	"effort":        {"low", "medium", "high", "max"},
	"notify":        {"off", "question", "done"},
	"notify_volume": {"20", "40", "60", "80", "100"},
	// One choice: the row only carries the number (the threshold in percent).
	"compact_percent": {"percent"},
	"notify_sound":    append([]string{"default", "bell"}, notify.Names()...),
}

func Choices(key string) []string {
	if c, ok := bashChoices[key]; ok {
		return c
	}
	return choices[key]
}

func (s *Settings) Get(key string) string {
	if value, ok := getBash(s, key); ok {
		return value
	}
	switch key {
	case "view":
		return s.View
	case "turn_summary":
		return s.TurnSummary
	case "effort":
		return s.Effort
	case "notify_sound":
		return s.NotifySound
	case "notify_volume":
		return strconv.Itoa(s.NotifyVolume)
	case "notify":
		return s.Notify
	case "notify_command":
		return s.NotifyCommand
	case "compact_percent":
		return "percent"
	case "system_prompt":
		if s.SystemPrompt == "" {
			return DefaultSystemPrompt
		}
		return s.SystemPrompt
	}
	return ""
}

func (s *Settings) Set(key, val string) error {
	if handled, err := setBashSetting(s, key, val); handled {
		return err
	}
	switch key {
	case "notify_command":
		s.NotifyCommand = strings.TrimSpace(val)
		return nil
	case "system_prompt":
		if val == "" || val == DefaultSystemPrompt {
			s.SystemPrompt = ""
			return nil
		}
		if err := ValidSystemPromptName(val); err != nil {
			return err
		}
		s.SystemPrompt = val
		return nil
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
	case "view":
		s.View = val
	case "turn_summary":
		s.TurnSummary = val
	case "effort":
		s.Effort = val
	case "notify_sound":
		s.NotifySound = val
	case "notify_volume":
		if n, err := strconv.Atoi(val); err == nil {
			s.NotifyVolume = n
		}
	case "notify":
		s.Notify = val
	}
}

// TextSetting reports whether a setting is free text instead of a choice.
func TextSetting(key string) bool {
	return key == "notify_command"
}

func (s *Settings) Cycle(key string, step int) string {
	if _, ok := getBash(s, key); !ok {
		if opts := choices[key]; len(opts) > 0 {
			cur := s.Get(key)
			index := 0
			for i, option := range opts {
				if option == cur {
					index = i
				}
			}
			index = ((index+step)%len(opts) + len(opts)) % len(opts)
			s.setRaw(key, opts[index])
			return opts[index]
		}
	}
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
