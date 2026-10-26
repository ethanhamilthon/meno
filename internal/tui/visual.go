package tui

import (
	"strings"
	"time"

	"meno/internal/render"
)

// transcriptWithSelection builds the timeline. Selections are painted by
// paintLog, using document positions rather than the old turn-level selection.
func (a *App) transcriptWithSelection(w int) []render.Line {
	if a.st.Session == nil {
		return nil
	}
	turns := a.st.Session.Turns()
	js := a.jsOpts()
	var out []render.Line
	separate := false
	appendLines := func(lines []render.Line, message bool) {
		if len(lines) == 0 {
			return
		}
		if len(out) > 0 && separate {
			out = append(out, render.Band(render.BgTerm))
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
			fg := render.FgDim
			if n.Err {
				fg = render.Red
			}
			appendLines(render.WrapLines(n.Text, render.TranscriptTextWidth(w), func(s string) render.Line {
				return render.Plain("  "+s, fg, render.BgTerm)
			}), true)
		}
		if i < len(turns) {
			appendLines(a.st.lines.turnLines(turns[i], w, js), render.TurnHasMessage(turns[i], js))
		}
	}

	// The runner persists a response before the UI consumes its completion
	// event. Hide its live copy as soon as the saved response is visible.
	persisted := false
	for i := max(0, a.st.LiveBase); i < len(turns); i++ {
		if turns[i].Role == render.RoleAssistant {
			persisted = true
			break
		}
	}
	if js.View == render.ViewCompact {
		if !persisted && a.st.LiveText != "" {
			appendLines(render.BuildLines([]render.Turn{{Role: render.RoleAssistant, Text: a.st.LiveText}}, w, js), true)
		}
		if a.st.Running {
			var elapsed time.Duration
			if !a.st.RunStart.IsZero() {
				elapsed = time.Since(a.st.RunStart)
			}
			appendLines([]render.Line{render.CompactLive(a.compactWhat(), elapsed, w)}, false)
		}
		return out
	}
	live := render.Turn{Role: render.RoleAssistant}
	if !persisted {
		live.Text = a.st.LiveText
		if a.st.LiveThink != "" {
			live.Thinking = a.st.LiveThink
		}
	}
	for _, call := range a.st.LiveCalls {
		if !call.Running && !call.Ended {
			continue
		}
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
		if call.Ended {
			// Drawn as the saved turn will be, so the hand-over changes nothing.
			live.Code, live.Title, live.Output, live.ElapsedMs = call.Code, call.Title, call.Output, call.ElapsedMs
			appendLines(render.BuildLines([]render.Turn{live}, w, js), render.TurnHasMessage(live, js))
			live = render.Turn{Role: render.RoleAssistant}
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
		appendLines(render.BuildLines([]render.Turn{live}, w, liveJS), render.TurnHasMessage(live, liveJS))
		live = render.Turn{Role: render.RoleAssistant}
	}
	appendLines(render.BuildLines([]render.Turn{live}, w, js), render.TurnHasMessage(live, js))
	return out
}

// compactWhat says what the running turn is doing, for the compact view: the
// start of the command being written or run, else "thinking...".
func (a *App) compactWhat() string {
	for i := len(a.st.LiveCalls) - 1; i >= 0; i-- {
		if c := a.st.LiveCalls[i]; !c.Ended && strings.TrimSpace(c.Code) != "" {
			return strings.TrimSpace(c.Code)
		}
	}
	if a.st.LiveThink != "" {
		return "thinking..."
	}
	return "working..."
}
