package config

import (
	"encoding/json"
	"strconv"
)

func applyLegacySettings(raw map[string]json.RawMessage, s *Settings) {
	copyString := func(key string, dst *string) {
		if b, ok := raw[key]; ok {
			_ = json.Unmarshal(b, dst)
		}
	}
	copyInt := func(key string, dst *int) {
		if b, ok := raw[key]; ok {
			_ = json.Unmarshal(b, dst)
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
	if b, ok := raw["js_code"]; ok {
		_ = json.Unmarshal(b, &s.BashCode)
	}
	if b, ok := raw["js_lines"]; ok {
		_ = json.Unmarshal(b, &s.BashLines)
	}
	if b, ok := raw["js_output"]; ok {
		_ = json.Unmarshal(b, &s.BashOutput)
	}
	if b, ok := raw["js_output_lines"]; ok {
		_ = json.Unmarshal(b, &s.BashOutputLines)
	}
	if b, ok := raw["js_strings"]; ok {
		_ = json.Unmarshal(b, &s.BashStrings)
	}
	if b, ok := raw["js_string_lines"]; ok {
		_ = json.Unmarshal(b, &s.BashStringLines)
	}
	if b, ok := raw["js_timing"]; ok {
		_ = json.Unmarshal(b, &s.BashTiming)
	}
	if s.JSTimeout == "sec" && s.JSTimeoutSec > 0 {
		s.BashTimeout = strconv.Itoa(s.JSTimeoutSec)
	} else if s.JSTimeout != "" {
		s.BashTimeout = s.JSTimeout
	}
}
