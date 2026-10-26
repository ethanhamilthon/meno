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
	js := a.jsOpts()
	out, separate := a.beginTranscript(w, js)
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
	// The runner persists a response before the UI consumes its completion
	// event. Hide its live copy as soon as the saved response is visible.
	persisted := a.st.transcript.lastAssistant >= max(0, a.st.LiveBase)
	if js.View == render.ViewCompact {
		if !persisted && a.st.LiveText != "" {
			appendLines(a.liveLines(render.Turn{Role: render.RoleAssistant, Text: a.st.LiveText}, w, js), true)
		}
		if a.st.Running {
			var elapsed time.Duration
			if !a.st.RunStart.IsZero() {
				elapsed = time.Since(a.st.RunStart)
			}
			line := render.CompactLive(a.compactWhat(), elapsed, w)
			if text := line.Text(); text != a.st.transcript.compactText {
				a.st.transcript.compactText = text
				a.st.transcript.changed = true
			}
			appendLines([]render.Line{line}, false)
		}
		return a.finishTranscript(out)
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
		saved := call.ID != "" && a.st.transcript.calls[call.ID]
		if saved {
			continue
		}
		if call.Ended {
			// Drawn as the saved turn will be, so the hand-over changes nothing.
			live.Code, live.Title, live.Output, live.ElapsedMs = call.Code, call.Title, call.Output, call.ElapsedMs
			appendLines(a.liveLines(live, w, js), render.TurnHasMessage(live, js))
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
		appendLines(a.liveLines(live, w, liveJS), render.TurnHasMessage(live, liveJS))
		live = render.Turn{Role: render.RoleAssistant}
	}
	appendLines(a.liveLines(live, w, js), render.TurnHasMessage(live, js))
	return a.finishTranscript(out)
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
