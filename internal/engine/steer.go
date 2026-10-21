package engine

import (
	"context"
	"fmt"
	"path/filepath"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/proc"
	"meno/internal/proto"
	"time"
)

// enqueueEvent stores a steering event for a session and wakes it when idle.
// acked means the model gets the content another way (a successful wait).
func (e *Engine) enqueueEvent(ev agent.Event, acked bool) {
	if e.events2 == nil {
		return
	}
	ok, err := e.events2.Enqueue(ev)
	if err != nil || !ok {
		return
	}
	if acked {
		_ = e.events2.Ack(ev.Session, ev.ID)
		return
	}
	e.wakeIfPending(ev.Session)
}

// eventSource is the steering queue of a session as the runner sees it.
func (e *Engine) eventSource(sid string) agent.EventSource {
	if e.events2 == nil {
		return nil
	}
	return agent.StoreSource{Store: e.events2, Session: sid}
}

// wakeIfPending starts a continuation of an idle session that has events
// waiting. Events of a cancelled request never wake it.
func (e *Engine) wakeIfPending(sid string) {
	s, err := e.load(context.Background(), sid)
	if err != nil || e.events2 == nil {
		return
	}
	reloadErr := e.reloadConfig()
	in := e.selectionInput()
	s.mu.Lock()
	if s.running || s.ask != nil || e.isClosed() {
		s.mu.Unlock()
		return
	}
	pending, err := e.events2.Pending(sid)
	if err != nil {
		s.mu.Unlock()
		return
	}
	var owner string
	found := false
	for _, ev := range pending {
		if s.cancelled[ev.Owner] {
			_ = e.events2.CancelOwner(sid, ev.Owner)
			continue
		}
		owner, found = ev.Owner, true
		break
	}
	if !found {
		s.mu.Unlock()
		return
	}
	if owner != "" {
		s.requestID = owner
	} else {
		s.requestID = agent.NewID()
	}
	// A continuation needs a usable model like a message does. Without one the
	// events stay pending and the next message or /model repairs the session.
	repaired := false
	if reloadErr == nil {
		repaired, reloadErr = resolveSelection(s, in)
	}
	if reloadErr != nil {
		s.mu.Unlock()
		e.publish(sid, proto.EventError, proto.ErrorData{Message: reloadErr.Error()})
		return
	}
	s.running, s.phase = true, "preparing"
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.live = proto.LiveState{Calls: []proto.CallState{}}
	s.mu.Unlock()
	e.publish(sid, proto.EventStatus, proto.StatusData{Running: true, Phase: "preparing"})
	if repaired {
		e.publish(sid, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"provider", "model", "effort"}})
	}
	e.launch(ctx, s, "", true)
}

func (e *Engine) isClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

// onTaskFinished turns a finished task into a steering event. A task the agent
// or a stop cancelled needs no news.
func (e *Engine) onTaskFinished(t proc.Task) {
	e.publish(t.Session, proto.EventTasksChanged, []string{t.ID})
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
		Text: fmt.Sprintf("Task %s (%s) finished: %s%s. Read its output with: meno tasks logs %s", t.ID, name, t.Status, code, t.ID),
		Refs: []string{"task:" + t.ID},
	}
	e.enqueueEvent(ev, e.tasks != nil && e.tasks.Waiting(t.ID))
}

func taskEventID(id string) string { return "task-" + id + "-done" }

// ackTask records that the model has seen the outcome of a task.
func (e *Engine) ackTask(session, id string) {
	if e.events2 != nil {
		_ = e.events2.Ack(session, taskEventID(id))
	}
}

var _ = llm.RoleUser
var _ = time.Now
