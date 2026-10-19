// Package tools provides standalone bash and restricted-command execution.
package tools

import (
	"fmt"
	"strconv"
	"strings"
)

// TimeoutMode describes how the bash timeout is selected.
type TimeoutMode string

const (
	// TimeoutSeconds uses the configured duration.
	TimeoutSeconds TimeoutMode = "seconds"
	// TimeoutOff disables the call timer.
	TimeoutOff TimeoutMode = "off"
	// TimeoutModel lets the model provide a timeout for each call.
	TimeoutModel TimeoutMode = "model"
)

// TimeoutSetting is a parsed bash_timeout setting. Seconds is set only for
// TimeoutSeconds.
type TimeoutSetting struct {
	Mode    TimeoutMode
	Seconds int
}

// ParseTimeoutSetting parses a positive number of seconds, "off", or "model".
// An empty setting selects the 120-second default.
func ParseTimeoutSetting(value string) (TimeoutSetting, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return TimeoutSetting{Mode: TimeoutSeconds, Seconds: 120}, nil
	}
	switch value {
	case string(TimeoutOff):
		return TimeoutSetting{Mode: TimeoutOff}, nil
	case string(TimeoutModel):
		return TimeoutSetting{Mode: TimeoutModel}, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 1 {
		return TimeoutSetting{}, fmt.Errorf("bash_timeout must be seconds, off, or model")
	}
	return TimeoutSetting{Mode: TimeoutSeconds, Seconds: seconds}, nil
}

// ToolDefinition is a model-facing tool definition without provider types.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// BashToolDefinition returns the bash tool schema. The timeout argument is
// present only when setting is TimeoutModel.
func BashToolDefinition(setting TimeoutSetting) ToolDefinition {
	properties := map[string]any{
		"command": map[string]any{"type": "string", "description": "Shell command to run."},
	}
	if setting.Mode == TimeoutModel {
		properties["timeout"] = map[string]any{"type": "integer", "minimum": 1, "description": "Maximum run time in seconds."}
	}
	return ToolDefinition{
		Name:        "bash",
		Description: "Run a shell command in the session working directory.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             []string{"command"},
			"additionalProperties": false,
		},
	}
}
