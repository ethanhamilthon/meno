package config

import (
	"encoding/json"
	"fmt"
	"strconv"
)

var bashChoices = map[string][]string{
	"tool": {"js", "bash"}, "bash_code": {"all", "lines", "off"},
	"bash_output": {"all", "lines", "off"},
	"bash_timing": {"all", "slow", "off"},
}

func hasLegacyJSSetting(raw map[string]json.RawMessage) bool {
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_timing"} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

// applyJSAliases reads the old js_* display keys. Execution timeouts and string
// folding no longer exist: their keys (js_timeout*, js_strings, bash_timeout,
// bash_strings, ...) are ignored.
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
	copyString("js_timing", "bash_timing", &s.BashTiming)
}

func normalizeBash(s *Settings, defaults Settings) {
	if s.BashLines < 1 || s.BashLines > MaxLines {
		s.BashLines = defaults.BashLines
	}
	if s.BashOutputLines < 1 || s.BashOutputLines > MaxLines {
		s.BashOutputLines = defaults.BashOutputLines
	}
	if s.DaemonIdleExitMin < 0 || s.DaemonIdleExitMin > 10080 {
		s.DaemonIdleExitMin = defaults.DaemonIdleExitMin
	}
	for key, value := range map[string]string{"bash_code": s.BashCode, "bash_output": s.BashOutput, "bash_timing": s.BashTiming} {
		if !contains(bashChoices[key], value) {
			setBash(s, key, defaults.Get(key))
		}
	}
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
	case "bash_timing":
		s.BashTiming = value
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
	case "bash_timing":
		return s.BashTiming, true
	case "daemon_idle_exit_min":
		return strconv.Itoa(s.DaemonIdleExitMin), true
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
	n, err := strconv.Atoi(value)
	if err != nil {
		return true, fmt.Errorf("invalid value %q for %s", value, key)
	}
	switch key {
	case "bash_lines", "bash_output_lines":
		if n < 1 || n > MaxLines {
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
