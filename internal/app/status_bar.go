package app

import (
	"strings"

	"meno/internal/ui"
)

// overlayLabels names the screen an open overlay shows in the badge.
var overlayLabels = map[Overlay]string{
	OverModel: "MODEL", OverSettings: "SETTINGS", OverLogin: "LOGIN", OverLoginFlow: "LOGIN",
	OverProviderEdit: "PROVIDER", OverSkills: "SKILLS", OverMCP: "MCP", OverMCPEdit: "MCP",
	OverContext: "CONTEXT", OverSessions: "SESSIONS", OverHelp: "HELP", OverFork: "FORK", OverTasks: "TASKS",
}

// statusLabel is the plain mode label, or the name of the open overlay.
func (a *App) statusLabel() string {
	if a.st.Over != OverNone {
		if name := overlayLabels[a.st.Over]; name != "" {
			return name
		}
	}
	return a.st.Mode.Label()
}

// statusLeft returns the badge text and the segments of the left side. The
// badge is independent of transcript and session usage.
func (a *App) statusLeft(w int) (string, []ui.Seg) {
	label := a.statusLabel()
	return label, []ui.Seg{{Text: " " + label + " ", FG: ui.FgBlack}}
}

// ctxLabel is the context size and the model's window, "12k/200k".
func (a *App) ctxLabel() string {
	max := "?"
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		max = compactTokens(info.Context)
	}
	return compactTokens(a.st.Context) + "/" + max
}

// statusRight is the right side: the model and its context use.
func (a *App) statusRight() []ui.Seg {
	if _, m, _ := a.selection(); m == "" {
		return []ui.Seg{{Text: a.modelLabel(), FG: ui.FgDim}, {Text: " ", FG: ui.FgDim}}
	}
	return []ui.Seg{{Text: a.modelLabel() + " ", FG: ui.FgDim}, {Text: a.ctxLabel(), FG: ui.FgDim}, {Text: " ", FG: ui.FgDim}}
}

// statusRightModel is the right side without the context figures.
func (a *App) statusRightModel() []ui.Seg {
	return []ui.Seg{{Text: a.modelLabel(), FG: ui.FgDim}, {Text: " ", FG: ui.FgDim}}
}

func (a *App) drawStatus(c ui.Canvas, x, y, w int) {
	text, left := a.statusLeft(w)
	mid := a.globalCounts()
	right := a.statusRight()
	if ui.TextWidth(joinSegText(mid))+ui.TextWidth(joinSegText(left))+ui.TextWidth(joinSegText(right))+4 > w {
		right = a.statusRightModel()
	}
	if ui.TextWidth(joinSegText(mid))+ui.TextWidth(joinSegText(left))+ui.TextWidth(joinSegText(right))+4 > w {
		right = nil
	}
	if ui.TextWidth(joinSegText(mid))+ui.TextWidth(joinSegText(left))+4 > w {
		for i := range mid {
			if mid[i].Text == " | " {
				continue
			}
			parts := strings.Fields(mid[i].Text)
			mid[i].Text = parts[0] + strings.ToLower(parts[1][:1])
		}
	}
	if len(mid) > 0 && ui.TextWidth(joinSegText(mid))+ui.TextWidth(joinSegText(left))+2 > w {
		mid = mid[len(mid)-1:]
	}
	ui.DrawBar(c, x, y, w, ui.BgTerm, left, mid, right)
	ui.Badge(c, x, y, text, a.st.Mode.Color())
}
