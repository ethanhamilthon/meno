package app

import (
	"fmt"
	"path/filepath"
	"time"

	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/proc"
)

// taskView is a task as the agent sees it (no environment, no secrets).
func taskView(t proc.Task) map[string]any {
	end := t.Ended
	if end.IsZero() {
		end = time.Now()
	}
	var code any
	if t.ExitCode != nil {
		code = *t.ExitCode
	}
	return map[string]any{
		"id": t.ID, "name": t.Name, "command": t.Command, "cwd": t.Cwd, "owner": t.Owner, "status": t.Status,
		"exitCode": code, "reason": t.Reason, "notify": t.Notify,
		"started": t.Started.Format(time.RFC3339), "ended": nilTime(t.Ended), "elapsedMs": end.Sub(t.Started).Milliseconds(),
	}
}

func nilTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// tasksHost implements jsvm.TasksHost for the main session sid.
type tasksHost struct {
	a   *App
	sid string
}

func (h tasksHost) owner() string {
	var o string
	h.a.onUI(func() {
		if rt := h.a.rts[h.sid]; rt != nil {
			o = rt.RequestID
		}
	})
	return o
}

func (h tasksHost) Start(c jsvm.TaskCall, command string) (any, error) {
	o := proc.StartOptions{Session: h.sid, Owner: h.owner(), Command: command, Cwd: c.Cwd, Env: c.Env, Notify: true}
	o.Name, _ = c.Opts["name"].(string)
	switch n := c.Opts["timeout"].(type) {
	case int64:
		o.Timeout = time.Duration(n) * time.Second
	case float64:
		o.Timeout = time.Duration(n * float64(time.Second))
	}
	if n, ok := c.Opts["notify"].(bool); ok {
		o.Notify = n
	}
	t, err := h.a.tasks.Start(o)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": t.ID}, nil
}

func (h tasksHost) List(c jsvm.TaskCall) (any, error) {
	all, _ := c.Opts["includeFinished"].(bool)
	ts, err := h.a.tasks.List(h.sid, all)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(ts))
	for i, t := range ts {
		out[i] = taskView(t)
	}
	return out, nil
}

func (h tasksHost) Status(_ jsvm.TaskCall, id string) (any, error) {
	t, err := h.a.tasks.Get(h.sid, id)
	if err != nil {
		return nil, err
	}
	return taskView(t), nil
}

func (h tasksHost) Logs(c jsvm.TaskCall, id string) (any, error) {
	o := proc.LogOptions{Tail: true}
	switch cur := c.Opts["cursor"].(type) {
	case int64:
		o.Cursor, o.Tail = cur, false
	case nil:
	default:
		return nil, fmt.Errorf("cursor must be the nextCursor number of an earlier read")
	}
	if n, ok := c.Opts["limit"].(int64); ok && n > 0 {
		o.Limit = int(n)
	}
	if s, ok := c.Opts["stream"].(string); ok {
		if s != "" && s != "all" && s != proc.Stdout && s != proc.Stderr {
			return nil, fmt.Errorf("stream must be all, stdout or stderr")
		}
		o.Stream = s
	}
	sl, err := h.a.tasks.Logs(h.sid, id, o)
	if err != nil {
		return nil, err
	}
	return toAny(sl)
}

func (h tasksHost) Wait(c jsvm.TaskCall, id string, timeout time.Duration) (any, error) {
	t, timedOut, err := h.a.tasks.Wait(c.Ctx, h.sid, id, timeout)
	if err != nil {
		return nil, err
	}
	v := taskView(t)
	v["waitTimedOut"] = timedOut
	if t.Terminal() && !timedOut && h.a.evStore != nil {
		_ = h.a.evStore.Ack(h.sid, taskEventID(id)) // the model has seen the outcome
	}
	return v, nil
}

func (h tasksHost) Cancel(_ jsvm.TaskCall, id string) (any, error) {
	if _, err := h.a.tasks.Cancel(h.sid, id); err != nil {
		return nil, err
	}
	t, _ := h.a.tasks.Get(h.sid, id)
	return taskView(t), nil
}

func taskEventID(id string) string { return "task-" + id + "-done" }

// onTaskFinished turns a finished task into a steering event. It runs on the
// supervisor's goroutine. A task the agent or :stop cancelled needs no news.
func (a *App) onTaskFinished(t proc.Task) {
	if !t.Notify || t.Status == proc.Cancelled {
		return
	}
	code := ""
	if t.ExitCode != nil {
		code = fmt.Sprintf(", exit code %d", *t.ExitCode)
	}
	name := t.Name
	if name == "" {
		name = filepath.Base(t.Command)
	}
	ev := agent.Event{
		ID: taskEventID(t.ID), Session: t.Session, Source: agent.SourceTask, From: t.ID, Kind: "task_done", Owner: t.Owner,
		Text: fmt.Sprintf("Task %s (%s) finished: %s%s. Read its output with tasks.logs(%q).", t.ID, name, t.Status, code, t.ID),
		Refs: []string{"task:" + t.ID},
	}
	// Someone is waiting for this task right now: the wait delivers the
	// outcome, so the notification is kept (for deduplication) but acknowledged.
	a.enqueueEventAcked(ev, a.tasks.Waiting(t.ID))
}
