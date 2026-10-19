package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// countGlyph is a state group of the status bar: the count, a Latin x and a
// diamond, with no spaces, for example "3x◆".
func countGlyph(n int) string { return itoa(n) + "x◆" }

// badgeText is the text of the colored badge: the mode label and, after it,
// the modes of the session in order, for example "NORMAL #subagents #goal".
func badgeText(label string, modes []string) string {
	for _, m := range modes {
		label += " #" + m
	}
	return label
}

// overlayLabels names the screen an open overlay shows in the badge.
var overlayLabels = map[Overlay]string{
	OverModel: "MODEL", OverSettings: "SETTINGS", OverLogin: "LOGIN", OverLoginFlow: "LOGIN",
	OverProviderEdit: "PROVIDER", OverSkills: "SKILLS", OverMCP: "MCP", OverMCPEdit: "MCP",
	OverSessions: "SESSIONS", OverHelp: "HELP", OverFork: "FORK", OverTasks: "TASKS",
	OverAgents: "AGENTS", OverSnippets: "SNIPPETS",
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

// statusGroups are the state groups of the left side, in order: the agent of
// this session, its background work, then a separator and the other sessions.
// The first group is green, the second purple (both blink), the done count is
// blue and does not blink.
func (a *App) statusGroups() []ui.Seg {
	var segs []ui.Seg
	add := func(text string, fg tcell.Color) {
		segs = append(segs, ui.Seg{Text: " ", FG: ui.FgDim}, ui.Seg{Text: text, FG: fg})
	}
	if a.st.Running {
		add(countGlyph(1), dimOr(blinkOn(), ui.Green))
	}
	if n := a.bgCount(a.st.Session.ID); n > 0 {
		add(countGlyph(n), bgColor())
	}
	running, _, done := a.otherSessions()
	if running > 0 || done > 0 {
		segs = append(segs, ui.Seg{Text: " |", FG: ui.FgDim})
	}
	if running > 0 {
		add(countGlyph(running), dimOr(blinkOn(), ui.Green))
	}
	if done > 0 {
		add(itoa(done)+"◉", ui.Blue)
	}
	return segs
}

// statusLeft returns the badge text and the segments of the left side. The
// modes leave the badge when the fixed parts would not fit next to the model.
func (a *App) statusLeft(w int) (string, []ui.Seg) {
	label := a.statusLabel()
	groups := a.statusGroups()
	text := label
	if a.st.Over == OverNone {
		text = badgeText(label, a.st.Session.Modes)
	}
	fixed := func(t string) int {
		return ui.TextWidth(" "+t+" ") + ui.TextWidth(joinSegText(groups)) + ui.TextWidth(a.modelLabel()) + 4
	}
	if fixed(text) > w {
		text = label
	}
	return text, append([]ui.Seg{{Text: " " + text + " ", FG: ui.FgBlack}}, groups...)
}

// statusRight is the right side: the model and nothing else.
func (a *App) statusRight() []ui.Seg {
	return []ui.Seg{{Text: a.modelLabel(), FG: ui.FgDim}, {Text: " ", FG: ui.FgDim}}
}

// statusCenter is the middle: usage, or a hint, a retry or a question.
func (a *App) statusCenter(room int) ui.Seg {
	center, color := a.statusUsage(room), ui.FgDim
	if hint := a.visualHint(); hint != "" {
		center = hint
	}
	if rt := a.retryText(); rt != "" {
		center, color = rt, ui.Yellow
	}
	if t := a.askStatus(); t != "" {
		center, color = t, ui.Orange
	}
	return ui.Seg{Text: center, FG: color}
}

func (a *App) drawStatus(c ui.Canvas, x, y, w int) {
	text, left := a.statusLeft(w)
	room := w - ui.TextWidth(joinSegText(left)) - ui.TextWidth(a.modelLabel()) - 4
	ui.DrawBar(c, x, y, w, ui.BgTerm, left, []ui.Seg{a.statusCenter(room)}, a.statusRight())
	ui.Badge(c, x, y, text, a.st.Mode.Color())
}
