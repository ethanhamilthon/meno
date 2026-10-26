package tui

import (
	"fmt"

	"meno/internal/render"
)

func (a *App) drawMain(c render.Canvas, w, h int) {
	bottom := min(h, a.bottomHeight())
	logH := h - bottom
	lines := a.transcriptWithSelection(w)
	if a.timelineEmpty() {
		a.drawWelcome(c, 0, 0, w, logH)
		// Notices on a fresh session sit at the bottom of the hero screen.
		for i := 0; i < len(lines) && i < logH; i++ {
			render.DrawLine(c, 0, logH-1-i, w, lines[len(lines)-1-i])
		}
	} else {
		a.drawLog(c, 0, 0, w, logH, lines)
	}
	if gap := a.logGap(); gap > 0 && bottom > gap {
		c.Fill(0, logH, w, gap, render.BgTerm)
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

func (a *App) bottomHeight() int { return a.bottomBase() }

// bottomBase is the bottom area: input, completions,
// status bar and the gap above them.
func (a *App) bottomBase() int {
	w, _ := a.scr.Size()
	return a.composerHeightFor(w) + a.completionHeight(w) + 1 + a.logGap()
}
func (a *App) drawLog(c render.Canvas, x, y, w, h int, lines []render.Line) {
	c.Fill(x, y, w, h, render.BgTerm)
	if a.st.Scroll > 0 && len(lines) > a.st.LogLines {
		a.st.Scroll += len(lines) - a.st.LogLines
	}
	a.st.LogLines = len(lines)
	a.st.Scroll = min(max(0, a.st.Scroll), max(0, len(lines)-h))
	start := max(0, len(lines)-h-a.st.Scroll)
	for i := 0; i < h && start+i < len(lines); i++ {
		render.DrawLine(c, x, y+i, w, lines[start+i])
	}
	a.paintLog(c, x, y, w, h, lines, start)
	if h > 0 && len(lines) > 0 && start+h >= len(lines) {
		for _, line := range lines[start:] {
			if line.CompletionAnchor {
				a.completionVisible = true
				break
			}
		}
	}
}
func (a *App) drawBottom(c render.Canvas, x, y, w, h int) {
	if h <= 0 {
		return
	}
	statusY := y + h - 1
	bodyH := max(0, h-1)
	if bodyH > 0 {
		ac := min(a.completionHeight(w), max(0, bodyH-a.composerHeightFor(w)))
		if ac > 0 && a.cmdActive() {
			a.drawCommandRows(c, x, y, w, ac)
		} else if ac > 0 && !a.acActive() {
			a.drawQueue(c, x, y, w, ac)
		} else if ac > 0 {
			a.drawAutocompleteRows(c, x, y, w, ac)
		}
		y += ac
		bodyH -= ac
		c.Fill(x, y, w, bodyH, render.BgTerm)
		a.drawComposer(c, x, y, w, bodyH)
	}
	a.drawStatus(c, x, statusY, w)
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

func joinSegText(segs []render.Seg) string {
	s := ""
	for _, g := range segs {
		s += g.Text
	}
	return s
}
