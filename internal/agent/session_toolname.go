package agent

import "meno/internal/llm"

// legacyToolName is the tool a call without a stored model message was made with.
func (s *Session) legacyToolName() string {
	if s.Tool == "bash" {
		return llm.BashToolName
	}
	return ""
}
