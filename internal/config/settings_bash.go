package config

import (
	"encoding/json"
	"fmt"
)

var bashChoices = map[string][]string{
	"tool": {"js", "bash"},
}

func hasLegacyJSSetting(raw map[string]json.RawMessage) bool {
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_timing"} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func setBash(s *Settings, key, value string) {
	switch key {
	case "tool":
		s.Tool = value
	}
}

func getBash(s *Settings, key string) (string, bool) {
	switch key {
	case "tool":
		return s.Tool, true
	}
	return "", false
}

func setBashSetting(s *Settings, key, value string) (bool, error) {
	if _, ok := getBash(s, key); !ok {
		return false, nil
	}
	if list, ok := bashChoices[key]; ok {
		if !contains(list, value) {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
		setBash(s, key, value)
		return true, nil
	}
	return true, fmt.Errorf("unknown setting %q", key)
}
