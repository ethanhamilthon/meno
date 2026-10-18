package app

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/proc"
	"meno/internal/ui"
)

// tasksView is the :tasks dialog: a snapshot of the current session's tasks,
// refreshed in the background, and the log of the opened one.
type tasksView struct {
	Rows     []proc.Task
	Logs     proc.LogSlice
	LogID    string // the task whose log is open ("" = the list)
	HideDone bool
	inflight bool
}

type tasksLoaded struct {
	sid  string
	rows []proc.Task
	logs proc.LogSlice
	id   string
}

func (a *App) openTasks() {
	if a.tasks == nil {
		a.notifyErr("tasks are not available")
		return
	}
	a.st.TasksUI = tasksView{}
	a.openOverlay(OverTasks)
	a.refreshTasks()
}

// refreshTasks reloads the snapshot off the UI goroutine; one load at a time.
func (a *App) refreshTasks() {
	if a.st.Over != OverTasks || a.st.TasksUI.inflight || a.tasks == nil {
		return
	}
	a.st.TasksUI.inflight = true
	sid, logID := a.st.Session.ID, a.st.TasksUI.LogID
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		rows, _ := a.tasks.List(sid, true)
		var logs proc.LogSlice
		if logID != "" {
			logs, _ = a.tasks.Logs(sid, logID, proc.LogOptions{Tail: true, Limit: 200})
		}
		a.postEvent(tasksLoaded{sid, rows, logs, logID})
	}()
}

func (a *App) onTasksLoaded(m tasksLoaded) {
	a.st.TasksUI.inflight = false
	if m.sid == a.st.Session.ID {
		a.st.TasksUI.Rows, a.st.TasksUI.Logs = m.rows, m.logs
	}
}

func (a *App) taskRows() []proc.Task {
	var out []proc.Task
	for _, t := range a.st.TasksUI.Rows {
		if !(a.st.TasksUI.HideDone && t.Terminal()) {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) tasksKey(k *tcell.EventKey) {
	v := &a.st.TasksUI
	rows := a.taskRows()
	if v.LogID != "" {
		return // logs are read-only; esc goes back
	}
	switch {
	case a.navKey(k, len(rows), true):
	case k.Key() == tcell.KeyEnter && len(rows) > 0:
		v.LogID = rows[clampSel(a.st.Sel, len(rows))].ID
		v.Logs = proc.LogSlice{}
		a.refreshTasks()
	case k.Rune() == 'c' && len(rows) > 0:
		t := rows[clampSel(a.st.Sel, len(rows))]
		if !t.Terminal() {
			_, _ = a.tasks.Cancel(t.Session, t.ID)
			a.notify("cancelling task " + t.ID)
		}
	case k.Rune() == 'f':
		v.HideDone = !v.HideDone
		a.st.Sel = 0
	}
}

func taskGlyph(t proc.Task) (string, tcell.Color) {
	switch t.Status {
	case proc.Starting, proc.Running:
		return "●", ui.Green
	case proc.Succeeded:
		return "✓", ui.FgDim
	case proc.Failed, proc.TimedOut:
		return "✗", ui.Red
	}
	return "○", ui.FgDim
}

func taskElapsed(t proc.Task) string {
	end := t.Ended
	if end.IsZero() {
		end = time.Now()
	}
	return clock(end.Sub(t.Started))
}

func (a *App) drawTasks(c ui.Canvas, w, h int) {
	v := a.st.TasksUI
	if v.LogID != "" {
		a.drawTaskLog(c, w, h)
		return
	}
	rows := a.taskRows()
	items := make([]item, 0, len(rows))
	for _, t := range rows {
		g, fg := taskGlyph(t)
		name := t.Name
		if name == "" {
			name = t.ID
		}
		right := t.Status + " " + taskElapsed(t)
		if t.ExitCode != nil {
			right += " exit " + itoa(*t.ExitCode)
		}
		it := item{glyph: g, glyphFG: fg, label: name + "  " + oneLine(t.Command), right: right}
		it.detail = shortPath(t.Cwd, 40) + " · request " + short8(t.Owner)
		items = append(items, it)
	}
	empty := "no tasks in this session"
	hint := "↑/↓ select · enter logs · c cancel · f hide finished · esc close"
	if v.HideDone {
		hint = "↑/↓ select · enter logs · c cancel · f show finished · esc close"
	}
	sel := clampSel(a.st.Sel, len(items))
	bh := 4 + itemsHeight(items, sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, 0, bh)
	b.frame(c)
	titleRow("tasks", itoa(len(items)))(c, b)
	if len(items) == 0 {
		b.put(c, 4, 2, empty, ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 2, items, sel)
	b.hint(c, hint)
}

func short8(s string) string {
	if len(s) > 8 {
		return s[len(s)-8:]
	}
	return s
}

func (a *App) drawTaskLog(c ui.Canvas, w, h int) {
	v := a.st.TasksUI
	var task proc.Task
	for _, t := range v.Rows {
		if t.ID == v.LogID {
			task = t
		}
	}
	b := centered(w, h, 0, max(6, h-4))
	b.frame(c)
	titleRow("task "+v.LogID+" · "+task.Status, taskElapsed(task))(c, b)
	b.put(c, 2, 1, ui.Truncate(oneLine(task.Command), max(1, b.w-4)), ui.FgDim, ui.BgPopup)
	room := max(0, b.h-4)
	recs := v.Logs.Records
	if len(recs) > room {
		recs = recs[len(recs)-room:]
	}
	if v.Logs.Lost > 0 || v.Logs.FirstSeq > 1 {
		b.put(c, 2, 2, "… older output was dropped to keep the log bounded", ui.FgDim, ui.BgPopup)
	}
	for i, r := range recs {
		fg := ui.FgSoft
		if r.Stream == proc.Stderr {
			fg = ui.Red
		}
		b.put(c, 2, 3+i, ui.Truncate(strings.ReplaceAll(r.Text, "\t", "    "), max(1, b.w-4)), fg, ui.BgPopup)
	}
	if len(v.Logs.Records) == 0 {
		b.put(c, 2, 3, "no output yet", ui.FgDim, ui.BgPopup)
	}
	b.hint(c, "live · esc back")
}
