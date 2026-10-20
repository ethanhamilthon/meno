package app

import (
	"time"

	"meno/internal/ui"
)

// transcriptWithSelection builds the timeline. Vim selections are painted by
// paintLog, using document positions rather than the old turn-level selection.
func (a *App) transcriptWithSelection(w int) []ui.Line {
	if a.st.Session == nil {
		return nil
	}
	turns := a.st.Session.Turns()
	js := a.jsOpts()
	var out []ui.Line
	separate := false
	appendLines := func(lines []ui.Line, message bool) {
		if len(lines) == 0 {
			return
		}
		if len(out) > 0 && separate {
			out = append(out, ui.Band(ui.BgTerm))
		}
		out = append(out, lines...)
		separate = message
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
			}), true)
		}
		if i < len(turns) {
			appendLines(a.st.lines.turnLines(turns[i], w, js), ui.TurnHasMessage(turns[i], js))
		}
	}

	// The runner persists a response before the UI consumes its completion
	// event. Hide its live copy as soon as the saved response is visible.
	persisted := false
	for i := max(0, a.st.LiveBase); i < len(turns); i++ {
		if turns[i].Role == ui.RoleAssistant {
			persisted = true
			break
		}
	}
	live := ui.Turn{Role: ui.RoleAssistant}
	if !persisted {
		live.Text = a.st.LiveText
		if a.st.LiveThink != "" {
			live.Thinking = a.st.LiveThink
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
		live.Code, live.Title = call.Code, call.Title
		liveJS := js
		liveJS.Running = true
		liveJS.Streaming = !call.Running // highlight only once the whole source has arrived
		if call.Running && !call.Started.IsZero() {
			liveJS.Exec = true
			liveJS.Elapsed = time.Since(call.Started)
			liveJS.Spin = int(time.Now().UnixMilli() / 100)
		}
		appendLines(ui.BuildLines([]ui.Turn{live}, w, liveJS), ui.TurnHasMessage(live, liveJS))
		live = ui.Turn{Role: ui.RoleAssistant}
	}
	appendLines(ui.BuildLines([]ui.Turn{live}, w, js), ui.TurnHasMessage(live, js))
	return out
}

func (a *App) visualHint() string {
	if a.st.Mode != ui.ModeVisual {
		return ""
	}
	return "hjkl move · y yank · esc back"
}
