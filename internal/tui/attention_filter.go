package tui

import "meno/internal/render"

// attention filters of /sessions, in the order Tab cycles through them.
const (
	filterAll = iota
	filterRunning
	filterAsking
	filterDone
	filterCount
)

var filterNames = [filterCount]string{"all", "running", "asking", "done"}

// attention is the state of a session as the counters of the status bar count
// it: asking wins over running, and done is a finished turn nobody has read.
type attention struct{ running, asking, done bool }

func (a *App) rowAttention(r sessionRow) attention {
	var at attention
	if r.rt != nil {
		at.asking = r.rt.Ask != nil
		at.running = r.rt.Running && !at.asking
		at.done = r.unseen || r.rt.Unseen
		return at
	}
	at.asking = r.needsYou
	at.running = r.running && !r.needsYou
	at.done = r.unseen
	return at
}

// matchesFilter says whether a row belongs to the chosen attention filter.
func (a *App) matchesFilter(r sessionRow, f int) bool {
	at := a.rowAttention(r)
	switch f {
	case filterRunning:
		return at.running
	case filterAsking:
		return at.asking
	case filterDone:
		return at.done
	}
	return true
}

// setFilter chooses a filter and puts the selection back on the first row.
func (a *App) setFilter(f int) {
	a.st.AttnFilter = ((f % filterCount) + filterCount) % filterCount
	a.st.Sel = 0
}

// openSessionsFiltered opens /sessions already filtered, as a click on a
// counter of the status bar does.
func (a *App) openSessionsFiltered(f int) {
	a.openSessions()
	a.setFilter(f)
}

// counterAt is the filter of the status-bar counter at column x, or -1.
func (a *App) counterAt(x int) int {
	pos := a.win.statusMidX
	for _, s := range a.win.statusMidSegs {
		w := barSegsWidth([]render.Seg{s})
		if x >= pos && x < pos+w {
			switch s.FG {
			case render.Green:
				return filterRunning
			case render.Orange:
				return filterAsking
			case render.Blue:
				return filterDone
			}
			return -1
		}
		pos += w
	}
	return -1
}
