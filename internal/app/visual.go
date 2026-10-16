package app

import (
	"meno/internal/agent"
	"meno/internal/ui"
)

// transcriptWithSelection builds transcript lines and, in visual mode, paints
// the lines belonging to the selected turn with the selection background.
func (a *App) transcriptWithSelection(w int) []ui.Line {
	if a.st.Session == nil {
		return nil
	}
	turns := a.st.Session.Snapshot()
	js := a.jsOpts()
	thinking := a.st.Settings.Thinking == "on"
	var out []ui.Line
	for i, t := range turns {
		lines := ui.BuildLines([]agent.Turn{t}, w, js, thinking)
		if a.st.Mode == ui.ModeVisual && i == a.st.Visual {
			for j := range lines {
				lines[j].BG = ui.BgSelect
			}
		}
		out = append(out, lines...)
	}
	return out
}

func (a *App) visualHint() string {
	if a.st.Mode != ui.ModeVisual {
		return ""
	}
	return "j/k select · y yank · r run · esc back"
}
