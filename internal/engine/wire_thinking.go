package engine

import (
	"strings"

	"meno/internal/agent"
)

// thinkingOf is the reasoning text stored with a turn, trimmed.
func thinkingOf(t agent.Turn) string {
	if t.Thinking != "" {
		return t.Thinking
	}
	if t.Replay != nil {
		return strings.TrimSpace(t.Replay.Thought)
	}
	return ""
}
