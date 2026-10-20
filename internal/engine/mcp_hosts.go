package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proc"
	"meno/internal/proto"
	"time"
)

type taskHost struct {
	e *Engine
	s *runState
}

func (h taskHost) Start(c jsvm.TaskCall, cmd string) (any, error) {
	o := c.Opts
	name, _ := o["name"].(string)
	cwd := c.Cwd
	if v, ok := o["cwd"].(string); ok && v != "" {
		cwd = v
	}
	env := make([]string, 0, len(c.Env)+len(h.s.env))
	for k, v := range h.s.env {
		env = append(env, k+"="+v)
	}
	env = append(env, c.Env...)
	if m, ok := o["env"].(map[string]any); ok {
		for k, v := range m {
			env = append(env, k+"="+fmt.Sprint(v))
		}
	}
	timeout := time.Duration(0)
	if n, ok := o["timeout"].(int64); ok {
		timeout = time.Duration(n) * time.Second
	}
	notify := true
	if v, ok := o["notify"].(bool); ok {
		notify = v
	}
	t, err := h.e.tasks.Start(proc.StartOptions{Session: h.s.session.ID, Owner: h.s.requestID, Name: name, Command: cmd, Cwd: cwd, Env: env, Timeout: timeout, Notify: notify})
	if err == nil {
		h.e.publish(h.s.session.ID, proto.EventTasksChanged, []string{t.ID})
	}
	return t, err
}
func (h taskHost) List(jsvm.TaskCall) (any, error) { return h.e.tasks.List(h.s.session.ID, true) }
func (h taskHost) Status(_ jsvm.TaskCall, id string) (any, error) {
	return h.e.tasks.Get(h.s.session.ID, id)
}
func (h taskHost) Logs(c jsvm.TaskCall, id string) (any, error) {
	o := proc.LogOptions{Tail: true}
	if n, ok := c.Opts["cursor"].(int64); ok {
		o.Cursor = n
		o.Tail = false
	}
	if n, ok := c.Opts["limit"].(int64); ok {
		o.Limit = int(n)
	}
	return h.e.tasks.Logs(h.s.session.ID, id, o)
}
func (h taskHost) Wait(c jsvm.TaskCall, id string, d time.Duration) (any, error) {
	t, _, err := h.e.tasks.Wait(c.Ctx, h.s.session.ID, id, d)
	return t, err
}
func (h taskHost) Cancel(_ jsvm.TaskCall, id string) (any, error) {
	t, err := h.e.tasks.Cancel(h.s.session.ID, id)
	if err == nil {
		h.e.publish(h.s.session.ID, proto.EventTasksChanged, []string{id})
	}
	return t, err
}

type mcpHost struct {
	e       *Engine
	s       *runState
	results mcp.ResultStore
}

func (h mcpHost) Tools() []jsvm.MCPTool {
	var out []jsvm.MCPTool
	for _, n := range h.e.mcp.Names() {
		for _, t := range h.e.mcp.ToolsOf(n) {
			out = append(out, jsvm.MCPTool{Server: n, Name: t.Name, Description: t.Description, Schema: t.InputSchema})
		}
	}
	return out
}
func (h mcpHost) Call(ctx context.Context, server, tool string, args map[string]any) (jsvm.MCPResult, error) {
	c, ok := h.e.mcp.Client(server)
	if !ok {
		return jsvm.MCPResult{}, fmt.Errorf("MCP server %q is not running", server)
	}
	text, err := c.Call(ctx, tool, args)
	if err != nil {
		return jsvm.MCPResult{}, err
	}
	res := mcp.Result{Text: text}
	saved, err := h.results.Save(h.s.session.ID, server, tool, res)
	if err != nil {
		return jsvm.MCPResult{}, err
	}
	return jsvm.MCPResult{Text: text, ResultID: saved.ID}, nil
}
func (h mcpHost) ResultInfo(id string) (any, error) { return h.results.Info(h.s.session.ID, id) }
func (h mcpHost) ResultRead(id string, off, limit int) (any, error) {
	return h.results.Read(h.s.session.ID, id, off, limit)
}

type mcpAdmin struct{ e *Engine }

func (h mcpAdmin) List() []map[string]any {
	out := []map[string]any{}
	for _, s := range h.e.servers {
		out = append(out, map[string]any{"name": s.Name, "enabled": s.Enabled, "running": h.e.mcp.Running(s.Name)})
	}
	return out
}
func (h mcpAdmin) Add(ctx context.Context, spec map[string]any) map[string]any {
	return h.mutate(ctx, "add", "", spec)
}
func (h mcpAdmin) Update(ctx context.Context, n string, p map[string]any) map[string]any {
	return h.mutate(ctx, "update", n, p)
}
func (h mcpAdmin) Remove(n string) map[string]any {
	return h.mutate(context.Background(), "remove", n, nil)
}
func (h mcpAdmin) Restart(ctx context.Context, n string) map[string]any {
	return h.mutate(ctx, "restart", n, nil)
}
func (h mcpAdmin) mutate(ctx context.Context, op, n string, spec map[string]any) map[string]any {
	return map[string]any{"ok": false, "error": "MCP administration is not configured"}
}

func encode(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func decodeData(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

var _ = llm.Usage{}
