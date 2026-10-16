package config

import "fmt"

type Settings struct {
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	JSCode      string `json:"js_code"`
	JSOutput    string `json:"js_output"`
	Thinking    string `json:"thinking"`
	JSTimeout   string `json:"js_timeout"`
	Permissions string `json:"permissions"`
}

func DefaultSettings() Settings {
	return Settings{
		Model:       "gpt-5.5",
		Effort:      "high",
		JSCode:      "full",
		JSOutput:    "on",
		Thinking:    "on",
		JSTimeout:   "30s",
		Permissions: "auto",
	}
}

var choices = map[string][]string{
	"js_code":     {"full", "lines5", "off"},
	"js_output":   {"on", "off"},
	"thinking":    {"on", "off"},
	"js_timeout":  {"5s", "30s", "none"},
	"permissions": {"auto", "ask"},
	"effort":      {"low", "medium", "high", "max"},
}

func SettingKeys() []string {
	return []string{"js_code", "js_output", "thinking", "js_timeout", "permissions"}
}

func Choices(key string) []string { return choices[key] }

func (s *Settings) Get(key string) string {
	switch key {
	case "js_code":
		return s.JSCode
	case "js_output":
		return s.JSOutput
	case "thinking":
		return s.Thinking
	case "js_timeout":
		return s.JSTimeout
	case "permissions":
		return s.Permissions
	case "effort":
		return s.Effort
	}
	return ""
}

func (s *Settings) Set(key, val string) error {
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
	case "js_code":
		s.JSCode = val
	case "js_output":
		s.JSOutput = val
	case "thinking":
		s.Thinking = val
	case "js_timeout":
		s.JSTimeout = val
	case "permissions":
		s.Permissions = val
	case "effort":
		s.Effort = val
	}
}

func (s *Settings) Cycle(key string, step int) string {
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
