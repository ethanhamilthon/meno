package app

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

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
	unseen         bool
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

// sessionRows merges the sessions the daemon listed with the ones followed in
// this run, newest first. Followed sessions win: they are ahead of the list.
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
		rows = append(rows, sessionRow{ID: rt.ID(), Title: rt.sessionTitle(), Cwd: rt.Cwd, UpdatedAt: updated, Turns: rt.Session.Len(), Model: a.runtimeModel(rt), unseen: rt.Unseen, rt: rt})
	}
	for _, s := range a.dbRows {
		if !seen[s.ID] {
			t, _ := time.Parse(time.RFC3339Nano, s.UpdatedAt)
			rows = append(rows, sessionRow{ID: s.ID, Title: s.Name, Cwd: s.Cwd, UpdatedAt: t, Model: s.Model, unseen: s.Completed > s.ReadThrough})
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
	q := a.st.Query.String()
	return rows, filterIdx(len(rows), func(i int) bool {
		s := rows[i]
		return containsFold(s.Title, q) || containsFold(s.Cwd, q) || containsFold(tildePath(s.Cwd), q) || containsFold(s.UpdatedAt.Format("2006-01-02"), q)
	})
}

func (a *App) sessionsKey(k *tcell.EventKey) {
	rows, idx := a.sessionsFiltered()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyCtrlU:
		a.st.Query.Reset()
		a.st.Sel = 0
	case k.Key() == tcell.KeyEnter:
		if len(idx) > 0 {
			a.openSession(rows[idx[clampSel(a.st.Sel, len(idx))]].ID)
		}
	default:
		a.queryKey(k)
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
	case rt.Unseen:
		return statusUnseen
	case rt.LastErr:
		return statusFailed
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
