package tui

import (
	"context"
	"encoding/json"

	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/render"
)

// tasksView is the /tasks dialog: the tasks of the current session as the
// daemon last said, and the log of the opened one.
type tasksView struct {
	Logs     []taskLogLine
	LogID    string // the task whose log is open ("" = the list)
	HideDone bool
	inflight bool
	// Up is how many rows the log is scrolled above its newest row. Zero means
	// follow: new output keeps the view at the end. Scrolling up stops
	// following until the user returns to the end with G or End.
	Up int
}

type taskLogLine struct {
	Text   string
	Stderr bool
}

type tasksLoaded struct {
	sid  string
	id   string
	logs []taskLogLine
}

func (a *App) openTasks() {
	a.st.TasksUI = tasksView{}
	a.openOverlay(OverTasks)
	a.refreshTasks()
}

// refreshTasks reloads the open log; session task lists arrive through the watcher.
func (a *App) refreshTasks() {
	if a.st.Over != OverTasks || a.st.TasksUI.inflight {
		return
	}
	a.st.TasksUI.inflight = true
	sid, logID := a.st.Session.ID(), a.st.TasksUI.LogID
	a.spawn(func() {
		var logs []taskLogLine
		if logID != "" {
			data, _ := json.Marshal(map[string]any{"id": logID, "limit": 200})
			_ = a.rpc(func(ctx context.Context, c *client.Client) error {
				r, e := c.Resource(ctx, "POST", "/v1/sessions/"+sid+"/tasks/logs", "", api.ResourceRequest{Data: data})
				if e != nil {
					return e
				}
				var out struct {
					Records []struct {
						Stream string `json:"stream"`
						Text   string `json:"text"`
					} `json:"records"`
				}
				_ = json.Unmarshal(r.Data, &out)
				for _, x := range out.Records {
					logs = append(logs, taskLogLine{Text: x.Text, Stderr: x.Stream == "stderr"})
				}
				return nil
			})
		}
		a.postEvent(tasksLoaded{sid: sid, id: logID, logs: logs})
	})
}

func (a *App) onTasksLoaded(m tasksLoaded) {
	a.st.TasksUI.inflight = false
	if m.sid == a.st.Session.ID() {
		a.st.TasksUI.Logs = m.logs
	}
}

func (a *App) taskRows() []taskRow {
	var out []taskRow
	for _, t := range a.st.Tasks {
		if !(a.st.TasksUI.HideDone && !taskRunning(t.Status)) {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) tasksKey(k *tcell.EventKey) {
	v := &a.st.TasksUI
	rows := a.taskRows()
	if v.LogID != "" {
		a.taskLogKey(k)
		return
	}
	switch {
	case a.navKey(k, len(rows), true):
	case k.Key() == tcell.KeyEnter && len(rows) > 0:
		v.LogID = rows[clampSel(a.st.Sel, len(rows))].ID
		v.Logs = nil
		a.refreshTasks()
	case isKey(k, 'c') && len(rows) > 0:
		t := rows[clampSel(a.st.Sel, len(rows))]
		if taskRunning(t.Status) {
			sid := a.st.Session.ID()
			a.spawn(func() {
				data, _ := json.Marshal(map[string]any{"id": t.ID})
				_ = a.rpc(func(ctx context.Context, c *client.Client) error {
					_, e := c.Resource(ctx, "POST", "/v1/sessions/"+sid+"/tasks/cancel", "", api.ResourceRequest{Data: data})
					return e
				})
			})
			a.notify("cancelling task " + t.ID)
		}
	case isKey(k, 'f'):
		v.HideDone = !v.HideDone
		a.st.Sel = 0
	}
}

func taskGlyph(t taskRow) (string, tcell.Color) {
	switch t.Status {
	case "starting", "running":
		return "●", render.Green
	case "succeeded":
		return "✓", render.FgDim
	case "failed", "timed_out", "timedout":
		return "✗", render.Red
	}
	return "○", render.FgDim
}

func (a *App) drawTasks(c render.Canvas, w, h int) {
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
		right := t.Status
		if t.Exit != nil {
			right += " exit " + itoa(*t.Exit)
		}
		items = append(items, item{glyph: g, glyphFG: fg, label: name + "  " + oneLine(t.Command), right: right})
	}
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
		b.put(c, 4, 2, "no tasks in this session", render.FgDim, render.BgPopup)
	}
	drawItems(c, b, 2, items, sel)
	b.hint(c, hint)
}

func (a *App) drawTaskLog(c render.Canvas, w, h int) {
	v := a.st.TasksUI
	var task taskRow
	for _, t := range a.st.Tasks {
		if t.ID == v.LogID {
			task = t
		}
	}
	b := centered(w, h, 0, max(6, h-4))
	b.frame(c)
	titleRow("task "+v.LogID+" · "+task.Status, "")(c, b)
	b.put(c, 2, 1, render.Truncate(oneLine(task.Command), max(1, b.w-4)), render.FgDim, render.BgPopup)
	room := max(0, b.h-4)
	rows := a.taskLogRows(max(1, b.w-4))
	end := len(rows) - min(v.Up, max(0, len(rows)-room))
	start := max(0, end-room)
	for i, r := range rows[start:end] {
		fg := render.FgSoft
		if r.err {
			fg = render.Red
		}
		b.put(c, 2, 3+i, r.text, fg, render.BgPopup)
	}
	if len(rows) == 0 {
		b.put(c, 2, 3, "no output yet", render.FgDim, render.BgPopup)
	}
	mode := "following"
	if v.Up > 0 {
		mode = itoa(v.Up) + " rows above the end"
	}
	b.hint(c, mode+" · ↑/↓ PgUp/PgDn scroll · G follow · y copy · esc back")
}
