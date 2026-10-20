package agent

import "meno/internal/llm"

// legacyJSTurn reports a tool turn made with the old js tool. The model no
// longer has that tool, so such a turn is sent as plain text ("ran js: …"),
// not as a tool call (the cache is cold once for an old session).
func (s *Session) legacyJSTurn(t Turn) bool {
	if s.toolName() != "js" || (t.CallID == "" && t.Code == "") {
		return false
	}
	if t.Replay == nil {
		return true
	}
	for _, c := range t.Replay.Calls() {
		if c.Name == llm.ToolName || c.Name == "" {
			return true
		}
	}
	return false
}
