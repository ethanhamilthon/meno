package engine

import (
	"meno/internal/llm"
)

func requestTokens(r *llm.Request) int {
	if r == nil {
		return 0
	}
	n := len(r.System) + len(r.ToolDescription)
	for _, m := range r.Msgs {
		n += len(m.Text) + len(m.Code) + len(m.Thought) + 1200*len(m.Images)
		for _, c := range m.ToolCalls {
			n += len(c.Name) + len(c.Arguments)
		}
		for _, raw := range m.Responses {
			n += len(raw)
		}
	}
	return n / 4
}
