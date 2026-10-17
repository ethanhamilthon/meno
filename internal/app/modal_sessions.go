package app

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/prompt"
	"meno/internal/ui"
)

// maxSessionRows is how many sessions the :sessions dialog lists.
const maxSessionRows = 200

// sessionRow is one line of the :sessions dialog: a session loaded in this
// run (rt is set; its live state is shown) or one that is only in the database.
type sessionRow struct {
	ID, Title, Cwd string
	UpdatedAt      time.Time
	Turns          int
	Model          string // "gpt-5.5 (openai, effort high)", "" when unknown
	rt             *Runtime
}

// describeSelection renders a model selection for the list, "" without a model.
func describeSelection(provider, model, effort string) string {
	if model == "" {
		return ""
	}
	return prompt.DescribeModel(provider, model, effort)
}

// runtimeModel is the model a loaded session will use for its next turn.
func (a *App) runtimeModel(rt *Runtime) string {
	s := rt.Session
	if s.Provider != "" && s.Model != "" {
		return describeSelection(s.Provider, s.Model, s.Effort)
	}
	st := a.st.Settings
	return describeSelection(st.Provider, st.Model, st.Effort)
}

// sessionRows merges the sessions of the database with the ones loaded in this
// run, newest first. Loaded sessions win: they are ahead of the database.
func (a *App) sessionRows() []sessionRow {
	seen := map[string]bool{}
	var rows []sessionRow
	for _, rt := range a.rtList {
		if rt.Empty() && rt != a.st.Runtime && !rt.Running {
			continue
		}
		seen[rt.ID()] = true
		updated := rt.Session.UpdatedAt
		if rt.Running {
			updated = time.Now()
		}
		rows = append(rows, sessionRow{ID: rt.ID(), Title: rt.sessionTitle(), Cwd: rt.Cwd, UpdatedAt: updated, Turns: rt.Session.Len(), Model: a.runtimeModel(rt), rt: rt})
	}
	for _, s := range a.dbRows {
		if !seen[s.ID] {
			rows = append(rows, sessionRow{ID: s.ID, Title: s.Title, Cwd: s.Cwd, UpdatedAt: s.UpdatedAt, Turns: s.Turns, Model: describeSelection(s.Provider, s.Model, s.Effort)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].UpdatedAt.After(rows[j].UpdatedAt) })
	if len(rows) > maxSessionRows {
		rows = rows[:maxSessionRows]
	}
	return rows
}

func (a *App) sessionsFiltered() ([]sessionRow, []int) {
	rows := a.sessionRows()
	q := string(a.st.Query)
	return rows, filterIdx(len(rows), func(i int) bool {
		s := rows[i]
		return containsFold(s.Title, q) || containsFold(s.Cwd, q) || containsFold(tildePath(s.Cwd), q) || containsFold(s.UpdatedAt.Format("2006-01-02"), q)
	})
}

// openSessions shows the dialog at once with what is known and lists the
// database in the background.
func (a *App) openSessions() {
	a.openOverlay(OverSessions)
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		list, err := agent.ListRecentSessions(a.paths.Home, maxSessionRows)
		a.postEvent(sessionsLoaded{list, err})
	}()
}

func (a *App) onSessionsLoaded(m sessionsLoaded) {
	if m.err != nil {
		a.notifyErr("sessions failed: " + m.err.Error())
		return
	}
	a.dbRows = m.list
}

func (a *App) sessionsKey(k *tcell.EventKey) {
	rows, idx := a.sessionsFiltered()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyCtrlU:
		a.st.Query = nil
		a.st.Sel = 0
	case k.Key() == tcell.KeyEnter:
		if len(idx) > 0 {
			a.openSession(rows[idx[clampSel(a.st.Sel, len(idx))]].ID)
		}
	default:
		a.queryKey(k)
	}
}

// openSession puts a session on screen: one loaded in this run as it is, a
// stored one freshly loaded with a new js VM. Running turns are left alone.
func (a *App) openSession(id string) {
	a.closeOverlay()
	if rt := a.rts[id]; rt != nil {
		a.switchTo(rt)
		return
	}
	s, err := agent.LoadSession(a.paths.Home, id)
	if err != nil {
		a.notifyErr("open failed: " + err.Error())
		return
	}
	cwd, storedCwd := s.Cwd, s.Cwd
	missing := cwd == "" || !dirExists(cwd)
	if missing {
		cwd = a.paths.Cwd
		s.Cwd = cwd
	}
	rt := a.newRuntime(s, cwd)
	rt.Saved = true
	rt.LastUsage, rt.Usage, rt.Spent, rt.Context, rt.Unpriced = s.Metrics.LastUsage, s.Metrics.Usage, s.Metrics.Spent, s.Metrics.Context, s.Metrics.Unpriced
	if !rt.Empty() {
		rt.Prompt = promptState{VMReset: true} // the VM is new; the prompt is built at the first message
		if missing && storedCwd != "" {
			rt.Prompt.CwdFrom = storedCwd
		}
	}
	a.switchTo(rt)
	a.notify("session opened")
	if missing {
		a.notifyErr("directory " + storedCwd + " no longer exists · using " + cwd)
	}
}

// tildePath shortens the home directory to ~.
func tildePath(p string) string {
	if h, err := os.UserHomeDir(); err == nil && h != "" && (p == h || strings.HasPrefix(p, h+string(os.PathSeparator))) {
		return "~" + p[len(h):]
	}
	return p
}

// shortPath keeps the tail of a path within n columns.
func shortPath(p string, n int) string {
	p = tildePath(p)
	if ui.TextWidth(p) <= n {
		return p
	}
	r := []rune(p)
	for len(r) > 1 && ui.TextWidth("…"+string(r)) > n {
		r = r[1:]
	}
	return "…" + string(r)
}

// sessionStatus is what a session is doing, most urgent first.
type sessionStatus int

const (
	statusIdle sessionStatus = iota
	statusUnseen
	statusFailed
	statusRetry
	statusRunning
	statusAsk
)

func (rt *Runtime) status() sessionStatus {
	switch {
	case rt.Ask != nil:
		return statusAsk
	case rt.Running && rt.Retry != nil:
		return statusRetry
	case rt.Running:
		return statusRunning
	case rt.LastErr:
		return statusFailed
	case rt.Unseen:
		return statusUnseen
	}
	return statusIdle
}

// blinkOn is the phase of the blinking dots: 500ms on, 500ms off.
func blinkOn() bool { return time.Now().UnixMilli()/500%2 == 0 }

// glyph is the status symbol and its color.
func (s sessionStatus) glyph() (string, tcell.Color) {
	dim := func(on bool, c tcell.Color) tcell.Color {
		if on {
			return c
		}
		return ui.FgDim
	}
	switch s {
	case statusAsk:
		return "?", ui.Orange
	case statusRetry:
		return "●", dim(blinkOn(), ui.Yellow)
	case statusRunning:
		return "●", dim(blinkOn(), ui.Green)
	case statusFailed:
		return "●", ui.Red
	case statusUnseen:
		return "●", ui.Blue
	}
	return "○", ui.FgDim
}

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
