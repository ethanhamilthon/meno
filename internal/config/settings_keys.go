package config

import (
	"fmt"
	"strconv"
	"strings"

	"meno/internal/notify"
)

var choices = map[string][]string{
	"images":        {"auto", "off", "kitty", "sixel", "iterm"},
	"thinking":      {"all", "lines", "off"},
	"turn_summary":  {"on", "off"},
	"effort":        {"low", "medium", "high", "max"},
	"notify":        {"off", "question", "done"},
	"notify_volume": {"20", "40", "60", "80", "100"},
	// One choice: the row only carries the number (the threshold in percent).
	"compact_percent": {"percent"},
	"notify_sound":    append([]string{"default", "bell"}, notify.Names()...),
}

func SettingKeys() []string {
	return append([]string{"theme", "images", "thinking", "turn_summary", "notify", "notify_sound", "notify_volume", "notify_command", "compact_percent"}, bashKeys...)
}

func Choices(key string) []string {
	if key == "bash_timeout" {
		return []string{"seconds", "off", "model"}
	}
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
	case "images":
		return s.ImagesProtocol
	case "thinking":
		return s.Thinking
	case "turn_summary":
		return s.TurnSummary
	case "effort":
		return s.Effort
	case "theme":
		return s.Theme
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
	case "notify_command":
		s.NotifyCommand = strings.TrimSpace(val)
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
	case "theme":
		s.Theme = val
	case "images":
		s.ImagesProtocol = val
	case "thinking":
		s.Thinking = val
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
	if key == "bash_timeout" {
		choices := []string{"seconds", "off", "model"}
		index := 0
		if n, err := strconv.Atoi(s.BashTimeout); err == nil {
			s.bashSeconds = n // remembered while the mode is off or model
		}
		if s.BashTimeout == "off" {
			index = 1
		} else if s.BashTimeout == "model" {
			index = 2
		}
		index = ((index+step)%len(choices) + len(choices)) % len(choices)
		if choices[index] == "seconds" {
			if _, err := strconv.Atoi(s.BashTimeout); err != nil {
				s.BashTimeout = "120"
				if s.bashSeconds > 0 {
					s.BashTimeout = strconv.Itoa(s.bashSeconds)
				}
			}
		} else {
			s.BashTimeout = choices[index]
		}
		return choices[index]
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
