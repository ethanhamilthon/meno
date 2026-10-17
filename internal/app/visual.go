package app

import (
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

// transcriptWithSelection builds the timeline. Vim selections are painted by
// paintLog, using document positions rather than the old turn-level selection.
func (a *App) transcriptWithSelection(w int) []ui.Line {
	if a.st.Session == nil {
		return nil
	}
	turns := a.st.Session.Snapshot()
	js := a.jsOpts()
	var out []ui.Line
	appendLines := func(lines []ui.Line) {
		if len(lines) == 0 {
			return
		}
		if len(out) > 0 {
			out = append(out, ui.Band(ui.BgTerm))
		}
		out = append(out, lines...)
	}
	notices := a.currentNotices()
	for i := 0; i <= len(turns); i++ {
		for _, n := range notices {
			if n.After != i {
				continue
			}
			fg := ui.FgDim
			if n.Err {
				fg = ui.Red
			}
			appendLines(ui.WrapLines(n.Text, max(1, w-2*ui.PadX-2), func(s string) ui.Line {
				return ui.Plain("  "+s, fg, ui.BgTerm)
			}))
		}
		if i < len(turns) {
			appendLines(ui.BuildLines(turns[i:i+1], w, js))
		}
	}

	// The runner persists a response before the UI consumes its completion
	// event. Hide its live copy as soon as the saved response is visible.
	persisted := false
	for i := max(0, a.st.LiveBase); i < len(turns); i++ {
		if turns[i].Role == llm.RoleAssistant {
			persisted = true
			break
		}
	}
	live := agent.Turn{Role: llm.RoleAssistant}
	if !persisted {
		live.Text = a.st.LiveText
		if a.st.LiveThink != "" {
			live.Replay = &llm.Msg{Thought: a.st.LiveThink}
		}
	}
	for _, call := range a.st.LiveCalls {
		saved := false
		for _, t := range turns {
			if call.ID != "" && t.CallID == call.ID {
				saved = true
				break
			}
		}
		if saved {
			continue
		}
		live.Code = call.Code
		liveJS := js
		liveJS.Running = true
		if call.Running && !call.Started.IsZero() {
			liveJS.Exec = true
			liveJS.Elapsed = time.Since(call.Started)
			liveJS.Limit = call.Limit
			liveJS.Spin = int(time.Now().UnixMilli() / 100)
		}
		appendLines(ui.BuildLines([]agent.Turn{live}, w, liveJS))
		live = agent.Turn{Role: llm.RoleAssistant}
	}
	appendLines(ui.BuildLines([]agent.Turn{live}, w, js))
	return out
}

func (a *App) visualHint() string {
	if a.st.Mode != ui.ModeVisual {
		return ""
	}
	return "hjkl move · y yank · esc back"
}
