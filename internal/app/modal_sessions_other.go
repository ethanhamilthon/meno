package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// otherSessions counts what the sessions other than the current one are doing.
func (a *App) otherSessions() (running, waiting, done int) {
	for _, rt := range a.rtList {
		if rt == a.st.Runtime {
			continue
		}
		switch rt.status() {
		case statusAsk:
			waiting++
		case statusRunning, statusRetry:
			running++
		}
		if rt.Unseen {
			done++
		}
	}
	return running, waiting, done
}

// sessionsSummary is the status-bar text about the other sessions, "" when
// none of them is active.
func (a *App) sessionsSummary() (string, tcell.Color) {
	running, waiting, done := a.otherSessions()
	var parts []string
	color := ui.Blue
	if running > 0 {
		parts = append(parts, "● "+itoa(running)+" running")
		color = dimOr(blinkOn(), ui.Green)
	}
	if waiting > 0 {
		parts = append(parts, "? "+itoa(waiting)+" waiting")
		color = ui.Orange
	}
	if done > 0 {
		parts = append(parts, "◉ "+itoa(done)+" done")
	}
	return strings.Join(parts, " · "), color
}

func dimOr(on bool, c tcell.Color) tcell.Color {
	if on {
		return c
	}
	return ui.FgDim
}

func (a *App) drawSessions(c ui.Canvas, w, h int) {
	rows, idx := a.sessionsFiltered()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		s := rows[i]
		g, gfg := "○", ui.FgDim
		if s.rt != nil {
			g, gfg = s.rt.status().glyph()
		}
		title := s.Title
		if title == "" {
			title = "untitled"
		}
		path := s.Cwd
		if path == "" {
			path = "path unavailable"
		}
		current := s.rt != nil && s.rt == a.st.Runtime
		when := ago(s.UpdatedAt)
		if current {
			when = "current"
		}
		turns := itoa(s.Turns) + " turns"
		// The title keeps at least 14 columns; the rest is filled as far as it fits.
		room := modalWidth(w) - 4 - 4 - 4 - min(14, ui.TextWidth(title))
		right := when
		for _, cand := range []string{turns + " · " + when, shortPath(path, 18) + " · " + turns + " · " + when} {
			if ui.TextWidth(cand) <= room {
				right = cand
			}
		}
		// The selected row adds what the row itself does not show: the full path and the model.
		detail := path
		if s.Model != "" {
			detail = s.Model + " · " + path // the model first: a long path gets cut, not the model
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: title, right: right, accent: current, detail: detail})
	}
	bh := 6 + itemsHeight(items, a.st.Sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, 0, bh)
	b.frame(c)
	titleRow("sessions", itoa(len(items))+" / "+itoa(len(rows)))(c, b)
	searchRow(c, b, 2, string(a.st.Query), "search title, date or path", "")
	if len(items) == 0 {
		b.put(c, 4, 4, "no matching sessions", ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 4, items, clampSel(a.st.Sel, len(items)))
	b.hint(c, "↑/↓ select · enter open · ctrl+u clear · esc close")
}
