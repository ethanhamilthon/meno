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

var bashKeys = []string{"tool", "bash_code", "bash_lines", "bash_output", "bash_output_lines", "bash_strings", "bash_string_lines", "bash_timing", "bash_timeout", "daemon_idle_exit_min"}

func hasLegacyJSSetting(raw map[string]json.RawMessage) bool {
	for _, key := range []string{"js_code", "js_output", "js_strings", "js_timing", "js_timeout"} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func copyJSSettings(raw map[string]json.RawMessage, s *Settings) {
	copyString := func(key string, dst *string) {
		if value, ok := raw[key]; ok {
			_ = json.Unmarshal(value, dst)
		}
	}
	copyInt := func(key string, dst *int) {
		if value, ok := raw[key]; ok {
			_ = json.Unmarshal(value, dst)
		}
	}
	copyString("js_code", &s.JSCode)
	copyInt("js_lines", &s.JSLines)
	copyString("js_output", &s.JSOutput)
	copyInt("js_output_lines", &s.JSOutLines)
	copyString("js_strings", &s.JSStrings)
	copyInt("js_string_lines", &s.JSStringLines)
	copyString("js_timing", &s.JSTiming)
	copyString("js_timeout", &s.JSTimeout)
	copyInt("js_timeout_sec", &s.JSTimeoutSec)
}

func applyJSAliases(raw map[string]json.RawMessage, s *Settings) {
	if hasLegacyJSSetting(raw) {
		copyJSSettings(raw, s)
		copyString := func(key string, dst *string) {
			if value, ok := raw[key]; ok {
				_ = json.Unmarshal(value, dst)
			}
		}
		copyInt := func(key string, dst *int) {
			if value, ok := raw[key]; ok {
				_ = json.Unmarshal(value, dst)
			}
		}
		copyString("js_code", &s.BashCode)
		copyInt("js_lines", &s.BashLines)
		copyString("js_output", &s.BashOutput)
		copyInt("js_output_lines", &s.BashOutputLines)
		copyString("js_strings", &s.BashStrings)
		copyInt("js_string_lines", &s.BashStringLines)
		copyString("js_timing", &s.BashTiming)
		if value, ok := raw["js_timeout"]; ok {
			var mode string
			if json.Unmarshal(value, &mode) == nil {
				if mode == "sec" {
					var n int
					copyInt("js_timeout_sec", &n)
					if n > 0 {
						s.BashTimeout = strconv.Itoa(n)
					}
				} else {
					s.BashTimeout = mode
				}
			}
		}
	}
}

func syncLegacyJSSettings(s *Settings) {
	s.JSCode, s.JSLines, s.JSOutput, s.JSOutLines = s.BashCode, s.BashLines, s.BashOutput, s.BashOutputLines
	s.JSStrings, s.JSStringLines, s.JSTiming = s.BashStrings, s.BashStringLines, s.BashTiming
	s.JSTimeout, s.JSTimeoutSec = "sec", 120
	if s.BashTimeout == "off" || s.BashTimeout == "model" {
		s.JSTimeout = s.BashTimeout
	} else if n, err := strconv.Atoi(s.BashTimeout); err == nil {
		s.JSTimeoutSec = n
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
		}
	}
}

func choicesForBash(key string) []string { return bashChoices[key] }
func canonicalSettingKey(key string) string {
	switch key {
	case "js_code":
		return "bash_code"
	case "js_output":
		return "bash_output"
	case "js_strings":
		return "bash_strings"
	case "js_timing":
		return "bash_timing"
	case "js_timeout":
		return "bash_timeout"
	}
	return key
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
		s.BashTimeout = value
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
		return s.BashTimeout, true
	case "daemon_idle_exit_min":
		return strconv.Itoa(s.DaemonIdleExitMin), true
	}
	return "", false
}

func setBashSetting(s *Settings, key, value string) (bool, error) {
	if key == "js_timeout" && s.Tool == "bash" {
		key = "bash_timeout"
	}
	if strings.HasPrefix(key, "js_") {
		return false, nil
	}
	if _, ok := getBash(s, key); !ok {
		return false, nil
	}
	value = strings.TrimSpace(value)
	if key == "bash_timeout" {
		if value == "off" || value == "model" {
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
