package app

import (
	"fmt"

	"meno/internal/agent"
	"meno/internal/ui"
)

// viewTurn converts a stored turn into what the transcript draws.
func viewTurn(t agent.Turn) ui.Turn {
	v := ui.Turn{
		ID: idOf(t.HID), CallID: t.CallID, Role: t.Role, Kind: t.Kind, Text: t.Text, Mode: t.Mode,
		Code: t.Code, Title: t.Title, Output: t.Output, Err: t.Err,
		ElapsedMs: t.ElapsedMs, LimitMs: t.LimitMs, TimedOut: t.TimedOut, Thinking: t.Thinking,
	}
	if v.Thinking == "" && t.Replay != nil {
		v.Thinking = t.Replay.Thought
	}
	for _, a := range t.Attachments {
		v.Attachments = append(v.Attachments, ui.Attachment{N: a.N, Mime: a.Mime, SHA: a.SHA, W: a.W, H: a.H})
	}
	if r := t.Run; r != nil {
		v.Run = &ui.RunStats{Status: r.Status, ElapsedMs: r.ElapsedMs, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMs: r.ToolMs,
			In: r.Usage.In, Out: r.Usage.Out, Cached: r.Usage.Cached, Cost: r.Cost, Err: r.Err}
	}
	return v
}

func viewTurns(ts []agent.Turn) []ui.Turn {
	out := make([]ui.Turn, len(ts))
	for i, t := range ts {
		out[i] = viewTurn(t)
	}
	return out
}

func idOf(hid int64) string {
	if hid == 0 {
		return ""
	}
	return fmt.Sprint(hid)
}
