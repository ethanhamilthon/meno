package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

var bashChoices = map[string][]string{
	"tool": {"js", "bash"}, "bash_code": {"all", "lines", "off"},
	"bash_output": {"all", "lines", "off"}, "bash_strings": {"off", "lines"},
	"bash_timing": {"all", "slow", "off"},
}

func hasLegacyJSSetting(raw map[string]json.RawMessage) bool {
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_strings", "js_string_lines", "js_timing", "js_timeout", "js_timeout_sec"} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func applyJSAliases(raw map[string]json.RawMessage, s *Settings) {
	copyString := func(old, current string, dst *string) {
		if _, exists := raw[current]; exists {
			return
		}
		if value, ok := raw[old]; ok {
			_ = json.Unmarshal(value, dst)
		}
	}
	copyInt := func(old, current string, dst *int) {
		if _, exists := raw[current]; exists {
			return
		}
		if value, ok := raw[old]; ok {
			_ = json.Unmarshal(value, dst)
		}
	}
	copyString("js_code", "bash_code", &s.BashCode)
	copyInt("js_lines", "bash_lines", &s.BashLines)
	copyString("js_output", "bash_output", &s.BashOutput)
	copyInt("js_output_lines", "bash_output_lines", &s.BashOutputLines)
	copyString("js_strings", "bash_strings", &s.BashStrings)
	copyInt("js_string_lines", "bash_string_lines", &s.BashStringLines)
	copyString("js_timing", "bash_timing", &s.BashTiming)
	if _, exists := raw["bash_timeout"]; exists {
		return
	}
	if _, ok := raw["js_timeout_sec"]; ok {
		if _, hasMode := raw["js_timeout"]; !hasMode {
			var seconds int
			if json.Unmarshal(raw["js_timeout_sec"], &seconds) == nil && seconds > 0 {
				s.BashTimeout = strconv.Itoa(min(seconds, MaxTimeoutSec))
			}
		}
	}
	var mode string
	if value, ok := raw["js_timeout"]; ok && json.Unmarshal(value, &mode) == nil {
		switch mode {
		case "":
			s.BashTimeout = "120"
		case "sec":
			var seconds int
			if value, ok := raw["js_timeout_sec"]; ok && json.Unmarshal(value, &seconds) == nil && seconds > 0 {
				s.BashTimeout = strconv.Itoa(min(seconds, MaxTimeoutSec))
			}
		case "off", "none":
			s.BashTimeout = "off"
		case "model":
			s.BashTimeout = "model"
		default:
			if seconds, err := strconv.Atoi(strings.TrimSuffix(mode, "s")); err == nil && seconds > 0 && strings.HasSuffix(mode, "s") {
				s.BashTimeout = strconv.Itoa(min(seconds, MaxTimeoutSec))
			}
		}
	}
}
func normalizeBash(s *Settings, defaults Settings) {
	if s.BashLines < 1 || s.BashLines > MaxLines {
		s.BashLines = defaults.BashLines
	}
	if s.BashOutputLines < 1 || s.BashOutputLines > MaxLines {
		s.BashOutputLines = defaults.BashOutputLines
	}
	if s.BashStringLines < 0 || s.BashStringLines > MaxLines {
		s.BashStringLines = defaults.BashStringLines
	}
	if s.DaemonIdleExitMin < 0 || s.DaemonIdleExitMin > 10080 {
		s.DaemonIdleExitMin = defaults.DaemonIdleExitMin
	}
	for key, value := range map[string]string{"bash_code": s.BashCode, "bash_output": s.BashOutput, "bash_strings": s.BashStrings, "bash_timing": s.BashTiming} {
		if !contains(choicesForBash(key), value) {
			setBash(s, key, defaults.Get(key))
		}
	}
	if s.BashTimeout != "off" && s.BashTimeout != "model" {
		n, err := strconv.Atoi(s.BashTimeout)
		if err != nil || n < 1 || n > MaxTimeoutSec {
			s.BashTimeout = defaults.BashTimeout
		} else {
			s.BashTimeout = strconv.Itoa(n)
		}
	}
}

func choicesForBash(key string) []string {
	if key == "bash_timeout" {
		return []string{"seconds", "off", "model"}
	}
	return bashChoices[key]
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
	if key == "js_timeout" {
		key = "bash_timeout"
	}
	switch key {
	case "tool":
		s.Tool = value
	case "bash_code":
		s.BashCode = value
	case "bash_lines":
		s.BashLines, _ = strconv.Atoi(value)
	case "bash_output":
		s.BashOutput = value
	case "bash_output_lines":
		s.BashOutputLines, _ = strconv.Atoi(value)
	case "bash_strings":
		s.BashStrings = value
	case "bash_string_lines":
		s.BashStringLines, _ = strconv.Atoi(value)
	case "bash_timing":
		s.BashTiming = value
	case "bash_timeout":
		if value == "seconds" {
			s.BashTimeout = "120"
			if n, err := strconv.Atoi(s.BashTimeout); err == nil && s.bashSeconds > 0 {
				s.BashTimeout = strconv.Itoa(s.bashSeconds)
				_ = n
			}
		} else {
			if n, err := strconv.Atoi(s.BashTimeout); err == nil {
				s.bashSeconds = n
			}
			s.BashTimeout = value
		}
	case "daemon_idle_exit_min":
		s.DaemonIdleExitMin, _ = strconv.Atoi(value)
	}
}

func getBash(s *Settings, key string) (string, bool) {
	switch key {
	case "tool":
		return s.Tool, true
	case "bash_code":
		return s.BashCode, true
	case "bash_lines":
		return strconv.Itoa(s.BashLines), true
	case "bash_output":
		return s.BashOutput, true
	case "bash_output_lines":
		return strconv.Itoa(s.BashOutputLines), true
	case "bash_strings":
		return s.BashStrings, true
	case "bash_string_lines":
		return strconv.Itoa(s.BashStringLines), true
	case "bash_timing":
		return s.BashTiming, true
	case "bash_timeout":
		if s.BashTimeout == "off" || s.BashTimeout == "model" {
			return s.BashTimeout, true
		}
		return "seconds", true
	case "daemon_idle_exit_min":
		return strconv.Itoa(s.DaemonIdleExitMin), true
	}
	return "", false
}

func setBashSetting(s *Settings, key, value string) (bool, error) {
	if key != "bash_timeout" {
		if _, ok := getBash(s, key); !ok {
			return false, nil
		}
	}
	if key == "bash_timeout" {
		value = strings.TrimSpace(value)
		if value == "seconds" {
			if s.BashTimeout == "off" || s.BashTimeout == "model" {
				s.BashTimeout = "120"
				if s.bashSeconds > 0 {
					s.BashTimeout = strconv.Itoa(s.bashSeconds)
				}
			}
			return true, nil
		}
		if value == "off" || value == "model" {
			if n, err := strconv.Atoi(s.BashTimeout); err == nil {
				s.bashSeconds = n
			}
			s.BashTimeout = value
			return true, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > MaxTimeoutSec {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
		s.BashTimeout = value
		return true, nil
	}
	if list, ok := bashChoices[key]; ok {
		if !contains(list, value) {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
		setBash(s, key, value)
		return true, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return true, fmt.Errorf("invalid value %q for %s", value, key)
	}
	switch key {
	case "bash_lines", "bash_output_lines":
		if n < 1 || n > MaxLines {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
	case "bash_string_lines":
		if n < 0 || n > MaxLines {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
	case "daemon_idle_exit_min":
		if n < 0 || n > 10080 {
			return true, fmt.Errorf("invalid value %q for %s", value, key)
		}
	}
	setBash(s, key, value)
	return true, nil
}
