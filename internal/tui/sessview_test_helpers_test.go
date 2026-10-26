package tui

import (
	"meno/internal/agent"
	"meno/internal/render"
)

// Append adds a turn for a test, the way the daemon's turn_added event would.
func (s *sessView) Append(t agent.Turn) {
	s.turns = append(s.turns, viewTurn(t)) // no id: laid out every time, like the old tests
}

// Replace swaps all turns for a test.
func (s *sessView) Replace(ts []agent.Turn) {
	s.turns = s.turns[:0]
	for _, t := range ts {
		s.Append(t)
	}
}

// viewTurn converts a stored turn into what the transcript draws (tests only).
func viewTurn(t agent.Turn) render.Turn {
	v := render.Turn{CallID: t.CallID, Role: t.Role, Kind: t.Kind, Text: t.Text, Code: t.Code, Title: t.Title,
		Output: t.Output, Err: t.Err, ElapsedMs: t.ElapsedMs, LimitMs: t.LimitMs, TimedOut: t.TimedOut, Thinking: t.Thinking}
	if v.Thinking == "" && t.Replay != nil {
		v.Thinking = t.Replay.Thought
	}
	if r := t.Run; r != nil {
		v.Run = &render.RunStats{Status: r.Status, ElapsedMs: r.ElapsedMs, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMs: r.ToolMs,
			In: r.Usage.In, Out: r.Usage.Out, Cached: r.Usage.Cached, CacheWrite: r.Usage.CacheWrite, CacheKnown: r.Usage.CacheReported, Cost: r.Cost, Partial: r.Partial, Err: r.Err}
	}
	return v
}
