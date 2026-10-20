package engine

import (
	"context"
	"meno/internal/jsvm"
	"meno/internal/proc"
	"meno/internal/proto"
	"net/url"
	"time"
)

func (e *Engine) taskResource(ctx context.Context, s *runState, tail []string, method string, data map[string]any, query string) (any, error) {
	op := "list"
	if len(tail) > 0 {
		op = tail[0]
	}
	id := stringValue(data, "id")
	if len(tail) > 1 {
		id = tail[1]
	}
	if id == "" {
		id = stringValue(data, "task_id")
	}
	switch op {
	case "", "list":
		all := true
		if q, er := url.ParseQuery(query); er == nil && q.Get("all") == "false" {
			all = false
		}
		return e.tasks.List(s.session.ID, all)
	case "start":
		cmd := stringValue(data, "command")
		cwd := stringValue(data, "cwd")
		if cwd == "" {
			cwd = s.cwd
		}
		timeout := time.Duration(number(data["timeout"])) * time.Second
		notify := boolValue(data, "notify", true)
		env := append(envSlice(s.env), mapEnv(data["env"])...)
		t, err := e.tasks.Start(proc.StartOptions{Session: s.session.ID, Owner: s.requestID, Name: stringValue(data, "name"), Command: cmd, Cwd: cwd, Env: env, Timeout: timeout, Notify: notify})
		if err == nil {
			e.publish(s.session.ID, proto.EventTasksChanged, []string{t.ID})
		}
		return t, err
	case "status":
		return e.tasks.Get(s.session.ID, id)
	case "logs":
		from := int64(number(data["from"]))
		limit := int(number(data["limit"]))
		return e.tasks.Logs(s.session.ID, id, proc.LogOptions{Cursor: from, Tail: from == 0, Limit: limit})
	case "wait":
		timeout := time.Duration(number(data["timeout"])) * time.Second
		t, to, err := e.tasks.Wait(ctx, s.session.ID, id, timeout)
		if err == nil && t.Terminal() && !to {
			e.ackTask(s.session.ID, id)
		}
		return map[string]any{"task": t, "waitTimedOut": to}, err
	case "cancel":
		t, err := e.tasks.Cancel(s.session.ID, id)
		if err == nil {
			e.publish(s.session.ID, proto.EventTasksChanged, []string{id})
		}
		return t, err
	}
	return nil, api(proto.CodeNotFound, "unknown task operation")
}
func (e *Engine) agentResource(ctx context.Context, s *runState, tail []string, method string, data map[string]any, query string) (any, error) {
	op := "list"
	if len(tail) > 0 {
		op = tail[0]
	}
	id := stringValue(data, "id")
	if len(tail) > 1 {
		id = tail[1]
	}
	host := childHost{e: e, parent: s}
	switch op {
	case "", "list":
		return host.List(jsvm.TaskCall{})
	case "spawn":
		return host.Spawn(jsvm.TaskCall{Ctx: ctx, Cwd: s.cwd, Env: envSlice(s.env), Opts: data}, data)
	case "status":
		return host.Status(jsvm.TaskCall{}, id)
	case "send":
		return host.Send(jsvm.TaskCall{}, id, stringValue(data, "text"))
	case "wait":
		return host.Wait(jsvm.TaskCall{Ctx: ctx}, id, time.Duration(number(data["timeout"]))*time.Second)
	case "result":
		return host.Result(jsvm.TaskCall{}, id)
	case "cancel":
		return host.Cancel(jsvm.TaskCall{}, id)
	case "report":
		return nil, host.Report(jsvm.TaskCall{}, stringValue(data, "text"))
	case "block":
		return nil, host.Block(jsvm.TaskCall{}, stringValue(data, "reason"))
	}
	return nil, api(proto.CodeNotFound, "unknown agent operation")
}
