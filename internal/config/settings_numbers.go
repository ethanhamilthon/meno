package config

import (
	"strconv"
	"time"
)

// MaxLines bounds the "lines <n>" limits and MaxTimeoutSec the bash timeout.
const (
	MaxLines      = 50
	MaxTimeoutSec = 3600
	// The auto-compact threshold, in percent of the context window.
	MinCompactPercent = 10
	MaxCompactPercent = 95
	compactStep       = 5
)

// NumOption is the choice of a setting that carries a number.
func NumOption(key string) string {
	switch key {
	case "bash_code", "bash_output", "bash_strings", "thinking":
		return "lines"
	case "bash_timeout":
		return "seconds"
	case "compact_percent":
		return "percent"
	}
	return ""
}

// NumFor returns the number that belongs to a setting's numeric choice.
func (s *Settings) NumFor(key string) int {
	switch key {
	case "bash_code":
		return s.BashLines
	case "bash_output":
		return s.BashOutputLines
	case "bash_strings":
		return s.BashStringLines
	case "bash_timeout":
		n, _ := strconv.Atoi(s.BashTimeout)
		return n
	case "thinking":
		return s.ThinkLines
	case "compact_percent":
		return s.CompactPercent
	default:
		return 0
	}
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
	if key == "bash_timeout" {
		if s.BashTimeout == "off" || s.BashTimeout == "model" {
			return 0
		}
	} else if NumOption(key) == "" {
		return 0
	}
	switch key {
	case "bash_timeout":
		n, err := strconv.Atoi(s.BashTimeout)
		if err != nil || n < 1 {
			return 0
		}
		if delta == 0 {
			return n
		}
		step := timeoutStep(n)
		if delta < 0 {
			step = timeoutStep(n - 1)
		}
		n = min(MaxTimeoutSec, max(1, n+delta*step))
		s.BashTimeout = strconv.Itoa(n)
		return n
	case "bash_code":
		s.BashLines = min(MaxLines, max(1, s.BashLines+delta))
		return s.BashLines
	case "bash_output":
		s.BashOutputLines = min(MaxLines, max(1, s.BashOutputLines+delta))
		return s.BashOutputLines
	case "bash_strings":
		s.BashStringLines = min(MaxLines, max(0, s.BashStringLines+delta))
		return s.BashStringLines
	case "compact_percent":
		s.CompactPercent = min(MaxCompactPercent, max(MinCompactPercent, s.CompactPercent+delta*compactStep))
		return s.CompactPercent
	case "thinking":
		s.ThinkLines = min(MaxLines, max(1, s.ThinkLines+delta))
		return s.ThinkLines
	default:
		return 0
	}
}

// DefaultTimeout applies when the model is in charge but names no timeout.
const DefaultTimeout = 30 * time.Second

// DefaultBashTimeout is the maximum run time when bash_timeout is unset.
const DefaultBashTimeout = 120 * time.Second

func (s Settings) BashTimeoutDuration() time.Duration {
	seconds, err := strconv.Atoi(s.BashTimeout)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if s.BashTimeout == "off" {
		return 0
	}
	if s.BashTimeout == "model" {
		return DefaultTimeout
	}
	return DefaultBashTimeout
}

// BashModelTimeout reports whether the model chooses the bash timeout per call.
func (s Settings) BashModelTimeout() bool { return s.BashTimeout == "model" }
