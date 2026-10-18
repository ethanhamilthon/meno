package app

import (
	"meno/internal/agent"
	"meno/internal/llm"
)

// planAnchor is how many turns come before the plan block. The plan sits right
// after the message that started the latest run, so the turns and the live
// reply streaming below it never move it (and it never jumps when a reply is
// saved). A session opened from disk uses its last user message.
func (a *App) planAnchor(turns []agent.Turn) int {
	if at := a.st.PlanAt; at > 0 && at <= len(turns) {
		return at
	}
	for i := len(turns) - 1; i >= 0; i-- {
		if t := turns[i]; t.Role == llm.RoleUser && t.Kind != agent.KindEvent && t.Kind != agent.KindView {
			return i + 1
		}
	}
	return len(turns)
}
