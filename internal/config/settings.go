package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	JSCode   string `json:"js_code"`
	JSLines  int    `json:"js_lines"`
	JSOutput string `json:"js_output"`
	// JSOutLines is the row limit of js output in "lines" mode.
	JSOutLines int `json:"js_output_lines"`
	// Thinking is all/lines/off like js code; ThinkLines is its row limit.
	Thinking   string `json:"thinking"`
	ThinkLines int    `json:"thinking_lines"`
	// JSTimeout is sec (JSTimeoutSec seconds), off (no limit) or model (the model
	// passes a timeout with each call).
	JSTimeout    string `json:"js_timeout"`
	JSTimeoutSec int    `json:"js_timeout_sec"`
	// Theme is the name of a ui theme; the app falls back to the default for
	// names it does not know.
	Theme string `json:"theme"`
}

// UnmarshalJSON restores safe defaults for obsolete enum values and invalid
// ranges. Decode into a copy so malformed JSON cannot partially alter settings.
func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	next := plain(DefaultSettings())
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	st := Settings(next)
	defaults := DefaultSettings()
	// Values from before js code/output shared the same all/lines/off choices.
	if st.JSCode == "full" {
		st.JSCode = "all"
	}
	switch st.JSOutput {
	case "on":
		st.JSOutput = "all"
	}
	if st.Thinking == "on" {
		st.Thinking = "all"
	}
	// Timeouts used to be 5s / 30s / none.
	if n, err := strconv.Atoi(strings.TrimSuffix(st.JSTimeout, "s")); err == nil && strings.HasSuffix(st.JSTimeout, "s") && n > 0 {
		st.JSTimeout, st.JSTimeoutSec = "sec", min(n, MaxTimeoutSec)
	} else if st.JSTimeout == "none" {
		st.JSTimeout = "off"
	}
	for key := range choices {
		if err := st.Set(key, st.Get(key)); err != nil {
			st.setRaw(key, defaults.Get(key))
		}
	}
	if st.Theme == "" {
		st.Theme = defaults.Theme
	}
	if st.JSLines < 1 || st.JSLines > MaxLines {
		st.JSLines = defaults.JSLines
	}
	if st.JSOutLines < 1 || st.JSOutLines > MaxLines {
		st.JSOutLines = defaults.JSOutLines
	}
	if st.ThinkLines < 1 || st.ThinkLines > MaxLines {
		st.ThinkLines = defaults.ThinkLines
	}
	if st.JSTimeoutSec < 1 || st.JSTimeoutSec > MaxTimeoutSec {
		st.JSTimeoutSec = defaults.JSTimeoutSec
	}
	*s = st
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Effort:       "high",
		JSCode:       "all",
		JSLines:      5,
		JSOutput:     "all",
		JSOutLines:   5,
		Thinking:     "all",
		ThinkLines:   5,
		Theme:        "meno",
		JSTimeout:    "sec",
		JSTimeoutSec: 30,
	}
}

var choices = map[string][]string{
	"js_code":    {"all", "lines", "off"},
	"js_output":  {"all", "lines", "off"},
	"thinking":   {"all", "lines", "off"},
	"js_timeout": {"sec", "off", "model"},
	"effort":     {"low", "medium", "high", "max"},
}

func SettingKeys() []string {
	return []string{"js_code", "js_output", "thinking", "js_timeout"}
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

// MaxLines bounds the "lines <n>" limits, MaxTimeoutSec the "<n> sec" timeout.
const (
	MaxLines      = 50
	MaxTimeoutSec = 3600
)

// NumOption is the choice of a setting that carries a number: "lines" for
// js_code, js_output and thinking, "sec" for js_timeout. Other settings return "".
func NumOption(key string) string {
	switch key {
	case "js_code", "js_output", "thinking":
		return "lines"
	case "js_timeout":
		return "sec"
	}
	return ""
}

// NumFor returns the number that belongs to a setting's numeric choice.
func (s *Settings) NumFor(key string) int {
	switch key {
	case "js_output":
		return s.JSOutLines
	case "thinking":
		return s.ThinkLines
	case "js_timeout":
		return s.JSTimeoutSec
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
