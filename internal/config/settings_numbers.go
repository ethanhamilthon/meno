package config

import (
	"strconv"
	"time"
)

// MaxLines bounds the "lines <n>" limits, MaxTimeoutSec the "<n> sec" timeout.
const (
	MaxLines      = 50
	MaxTimeoutSec = 3600
	// The auto-compact threshold, in percent of the context window.
	MinCompactPercent = 10
	MaxCompactPercent = 95
	compactStep       = 5
)

// NumOption is the choice of a setting that carries a number: "lines" for
// js_code, js_output and thinking, "sec" for js_timeout. Other settings return "".
func NumOption(key string) string {
	if key == "js_timing" {
		return ""
	}
	if key == "bash_code" || key == "bash_output" || key == "bash_strings" {
		return "lines"
	}
	if key == "bash_timeout" {
		return "sec"
	}
	if key == "js_code" || key == "js_output" || key == "js_strings" {
		return "lines"
	}
	if key == "js_timeout" {
		return "sec"
	}
	switch key {
	case "js_code", "js_output", "js_strings", "thinking":
		return "lines"
	case "js_timeout":
		return "sec"
	case "compact_percent":
		return "percent"
	}
	return ""
}

// NumFor returns the number that belongs to a setting's numeric choice.
func (s *Settings) NumFor(key string) int {
	if key == "bash_code" {
		return s.BashLines
	}
	if key == "bash_output" {
		return s.BashOutputLines
	}
	if key == "bash_strings" {
		return s.BashStringLines
	}
	if key == "bash_timeout" {
		n, _ := strconv.Atoi(s.BashTimeout)
		return n
	}
	switch key {
	case "js_strings":
		return s.JSStringLines
	case "js_output":
		return s.JSOutLines
	case "thinking":
		return s.ThinkLines
	case "js_timeout":
		return s.JSTimeoutSec
	case "compact_percent":
		return s.CompactPercent
	}
	return s.JSLines
}

// timeoutStep is the size of one +/- press at n seconds: fine at the start,
// coarser for long timeouts.
func timeoutStep(n int) int {
	switch {
	case n < 10:
		return 1
	case n < 60:
		return 5
	case n < 600:
		return 30
	}
	return 300
}

// AdjustNum changes the number of a setting's numeric choice by one step in the
// direction of delta and returns the new value.
func (s *Settings) AdjustNum(key string, delta int) int {
	if key == "js_timeout" && s.Tool == "bash" {
		key = "bash_timeout"
	}
	if (key == "js_code" || key == "js_output" || key == "js_strings") && s.Tool == "bash" {
		key = canonicalSettingKey(key)
	}
	if key == "bash_timeout" {
		n, _ := strconv.Atoi(s.BashTimeout)
		step := timeoutStep(n)
		if delta < 0 {
			step = timeoutStep(n - 1)
		}
		n = min(MaxTimeoutSec, max(1, n+delta*step))
		s.BashTimeout = strconv.Itoa(n)
		return n
	}
	if key == "bash_code" {
		s.BashLines = min(MaxLines, max(1, s.BashLines+delta))
		return s.BashLines
	}
	if key == "bash_output" {
		s.BashOutputLines = min(MaxLines, max(1, s.BashOutputLines+delta))
		return s.BashOutputLines
	}
	if key == "bash_strings" {
		s.BashStringLines = min(MaxLines, max(0, s.BashStringLines+delta))
		return s.BashStringLines
	}
	if key == "js_timeout" {
		n := s.JSTimeoutSec
		switch {
		case delta > 0:
			n += timeoutStep(n)
		case delta < 0:
			n -= timeoutStep(n - 1)
		}
		s.JSTimeoutSec = min(MaxTimeoutSec, max(1, n))
		return s.JSTimeoutSec
	}
	if key == "compact_percent" {
		s.CompactPercent = min(MaxCompactPercent, max(MinCompactPercent, s.CompactPercent+delta*compactStep))
		return s.CompactPercent
	}
	if key == "js_strings" {
		s.JSStringLines = min(MaxLines, max(0, s.JSStringLines+delta))
		return s.JSStringLines
	}
	p := &s.JSLines
	switch key {
	case "js_output":
		p = &s.JSOutLines
	case "thinking":
		p = &s.ThinkLines
	}
	*p = min(MaxLines, max(1, *p+delta))
	return *p
}

// DefaultTimeout applies when the timeout setting says nothing usable and
// when the model is in charge but names no timeout.
const DefaultTimeout = 30 * time.Second

func (s Settings) TimeoutDuration() time.Duration {
	switch s.JSTimeout {
	case "sec":
		if s.JSTimeoutSec > 0 {
			return time.Duration(s.JSTimeoutSec) * time.Second
		}
	case "off":
		return 0
	}
	return DefaultTimeout
}

// ModelTimeout reports whether the model chooses the js timeout per call.
func (s Settings) ModelTimeout() bool { return s.JSTimeout == "model" }

// BashModelTimeout reports whether the model chooses the bash timeout per call.
func (s Settings) BashModelTimeout() bool { return s.BashTimeout == "model" }
