package app

import (
	"fmt"
	"time"

	"meno/internal/ui"
)

func (a *App) drawMain(c ui.Canvas, w, h int) {
	bottom := min(h, a.bottomHeight())
	logH := h - bottom
	lines := a.transcriptWithSelection(w)
	if a.timelineEmpty() {
		a.drawWelcome(c, 0, 0, w, logH)
		// Notices on a fresh session sit at the bottom of the hero screen.
		for i := 0; i < len(lines) && i < logH; i++ {
			ui.DrawLine(c, 0, logH-1-i, w, lines[len(lines)-1-i])
		}
	} else {
		a.drawLog(c, 0, 0, w, logH, lines)
	}
	if gap := a.logGap(); gap > 0 && bottom > gap {
		c.Fill(0, logH, w, gap, ui.BgTerm)
		a.drawBottom(c, 0, logH+gap, w, bottom-gap)
		return
	}
	a.drawBottom(c, 0, logH, w, bottom)
}

// logGap is the blank row between the last block of the timeline and the
// input, so the last message does not touch the input. It is part of the
// bottom area, so every scroll and cursor computation already accounts for it.
func (a *App) logGap() int {
	if a.timelineEmpty() && len(a.currentNotices()) == 0 {
		return 0
	}
	return 1
}

const commandMaxRows = 25

func (a *App) bottomHeight() int {
	if a.st.Mode == ui.ModeCommand {
		w, _ := a.scr.Size()
		return acRowCount(len(commandRows(a)), w) + 1 + a.logGap()
	}
	return a.composerHeight() + a.acHeight() + 1 + a.logGap()
}
func (a *App) drawLog(c ui.Canvas, x, y, w, h int, lines []ui.Line) {
	c.Fill(x, y, w, h, ui.BgTerm)
	if a.st.Scroll > 0 && len(lines) > a.st.LogLines {
		a.st.Scroll += len(lines) - a.st.LogLines
	}
	a.st.LogLines = len(lines)
	a.st.Scroll = min(max(0, a.st.Scroll), max(0, len(lines)-h))
	start := max(0, len(lines)-h-a.st.Scroll)
	for i := 0; i < h && start+i < len(lines); i++ {
		ui.DrawLine(c, x, y+i, w, lines[start+i])
	}
	a.paintLog(c, x, y, w, h, lines, start)
}
func (a *App) drawBottom(c ui.Canvas, x, y, w, h int) {
	if h <= 0 {
		return
	}
	if a.st.Mode == ui.ModeCommand {
		// Command mode owns the whole bottom: completion rows and one input line.
		rows := min(acRowCount(len(commandRows(a)), w), h-1)
		a.drawCommandRows(c, x, y, w, rows)
		a.drawCommandLine(c, x, y+h-1, w)
		return
	}
	statusY := y + h - 1
	bodyH := max(0, h-1)
	if bodyH > 0 {
		ac := min(a.acHeight(), max(0, bodyH-a.composerHeight()))
		if ac > 0 && a.st.Ask != nil {
			a.drawAsk(c, x, y, w, ac)
		} else if ac > 0 {
			a.drawAutocompleteRows(c, x, y, w, ac)
		}
		y += ac
		bodyH -= ac
		c.Fill(x, y, w, bodyH, ui.BgUser)
		a.drawComposer(c, x, y, w, bodyH)
	}
	a.drawStatus(c, x, statusY, w)
}
func (a *App) drawStatus(c ui.Canvas, x, y, w int) {
	label := a.st.Mode.Label()
	if a.st.Over != OverNone {
		labels := map[Overlay]string{OverModel: "MODEL", OverSettings: "SETTINGS", OverProviders: "PROVIDERS", OverProviderEdit: "PROVIDER", OverSkills: "SKILLS", OverMCP: "MCP", OverMCPEdit: "MCP", OverSessions: "SESSIONS", OverTheme: "THEME", OverHelp: "HELP"}
		if name := labels[a.st.Over]; name != "" {
			label = name
		}
	}
	left := []ui.Seg{{Text: " " + label + " ", FG: ui.FgBlack}}
	center := a.statusUsageText()
	if hint := a.visualHint(); hint != "" {
		center = hint
	}
	midColor := ui.FgDim
	if rt := a.retryText(); rt != "" {
		center, midColor = rt, ui.Yellow
	}
	if t := a.askStatus(); t != "" {
		center, midColor = t, ui.Orange
	}
	mid := []ui.Seg{{Text: center, FG: midColor}}
	if sum, col := a.sessionsSummary(); sum != "" {
		// What the other sessions are doing comes first; the usage text only if it fits.
		both := []ui.Seg{{Text: sum, FG: col}, {Text: "   ", FG: ui.FgDim}, {Text: center, FG: midColor}}
		if center != "" && ui.TextWidth(joinSegText(both))+ui.TextWidth(a.modelLabel())+ui.TextWidth(label)+10 <= w {
			mid = both
		} else {
			mid = []ui.Seg{{Text: sum, FG: col}}
		}
	}
	dot, color := "○ ", ui.FgDim
	if a.st.Running {
		// Blink while the agent works: 500ms on, 500ms off.
		dot, color = "● ", ui.Green
		if time.Now().UnixMilli()/500%2 == 1 {
			color = ui.FgDim
		}
	}
	right := []ui.Seg{{Text: dot, FG: color}, {Text: a.modelLabel(), FG: ui.FgDim}, {Text: " ", FG: ui.FgDim}}
	ui.DrawBar(c, x, y, w, ui.BgTerm, left, mid, right)
	ui.Badge(c, x, y, label, a.st.Mode.Color())
}

func compactTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fm", float64(n)/1000000)
	}
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprint(n)
}

func (a *App) contextLabel() string {
	text := compactTokens(a.st.Context)
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		text += " / " + compactTokens(info.Context)
	}
	return text
}

func joinSegText(segs []ui.Seg) string {
	s := ""
	for _, g := range segs {
		s += g.Text
	}
	return s
}
