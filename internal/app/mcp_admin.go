package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"meno/internal/mcp"
)

// startWait is how long add/update/restart wait for a server to come up.
const startWait = 30 * time.Second

// uiCall runs fn on the UI goroutine, which owns a.st.
type uiCall struct {
	fn   func()
	done chan struct{}
}

// onUI runs fn on the UI goroutine and waits for it. It reports false when the
// app is closing and fn did not run.
func (a *App) onUI(fn func()) bool {
	c := uiCall{fn, make(chan struct{})}
	a.postEvent(c)
	select {
	case <-c.done:
		return true
	case <-a.done:
		return false
	}
}

// mcpAdmin implements jsvm.MCPAdmin on top of the app state, so that changes
// made by the agent and by the :mcp dialogs share one source of truth.
type mcpAdmin struct {
	a   *App
	sid string // the session whose agent calls us; its timeline gets the notices
}

func fail(format string, args ...any) map[string]any {
	return map[string]any{"ok": false, "error": fmt.Sprintf(format, args...)}
}

func mask(m map[string]string) map[string]any {
	out := map[string]any{}
	for k := range m {
		out[k] = "***"
	}
	return out
}

func (m mcpAdmin) status(s mcp.Server) map[string]any {
	a := m.a
	status, errText := "stopped", ""
	switch {
	case a.mcp.Starting(s.Name):
		status = "starting"
	case a.mcp.Running(s.Name):
		status = "running"
	case a.mcp.Err(s.Name) != "":
		status, errText = "failed", a.mcp.Err(s.Name)
	}
	out := map[string]any{
		"name": s.Name, "transport": s.Transport, "enabled": s.Enabled, "status": status,
		"tools": len(a.mcp.ToolsOf(s.Name)),
	}
	if s.Command != "" {
		out["command"] = s.Command
	}
	if s.URL != "" {
		out["url"] = s.URL
	}
	if len(s.Env) > 0 {
		out["env"] = mask(s.Env)
	}
	if len(s.Headers) > 0 {
		out["headers"] = mask(s.Headers)
	}
	if errText != "" {
		out["error"] = errText
	}
	return out
}

func (m mcpAdmin) List() []map[string]any {
	out := []map[string]any{}
	m.a.onUI(func() {
		for _, s := range m.a.st.Servers {
			out = append(out, m.status(s))
		}
	})
	return out
}

// applySpec copies the fields of a js object onto s. Unknown fields and wrong
// types are errors, so that typos do not silently do nothing.
func applySpec(s *mcp.Server, spec map[string]any) error {
	strs := func(k string, v any) (map[string]string, error) {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be an object of strings", k)
		}
		out := map[string]string{}
		for kk, vv := range m {
			str, ok := vv.(string)
			if !ok {
				return nil, fmt.Errorf("%s.%s must be a string", k, kk)
			}
			out[kk] = str
		}
		return out, nil
	}
	keys := make([]string, 0, len(spec))
	for k := range spec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := spec[k]
		var err error
		str, isStr := v.(string)
		switch k {
		case "name", "transport", "command", "url":
			if !isStr {
				return fmt.Errorf("%s must be a string", k)
			}
			switch k {
			case "name":
				s.Name = str
			case "transport":
				s.Transport = str
			case "command":
				s.Command = str
			case "url":
				s.URL = str
			}
		case "enabled":
			b, ok := v.(bool)
			if !ok {
				return errors.New("enabled must be a boolean")
			}
			s.Enabled = b
		case "env":
			s.Env, err = strs(k, v)
		case "headers":
			s.Headers, err = strs(k, v)
		default:
			return fmt.Errorf("unknown field %q (allowed: name, transport, command, url, env, headers, enabled)", k)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m mcpAdmin) find(name string) int {
	for i, s := range m.a.st.Servers {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// change applies fn to the server list on the UI goroutine, saves it and keeps
// the open dialogs consistent. fn returns the new list and a timeline message.
func (m mcpAdmin) change(fn func(sv []mcp.Server) ([]mcp.Server, string, error)) error {
	var err error
	ran := m.a.onUI(func() {
		a := m.a
		old := append([]mcp.Server(nil), a.st.Servers...)
		var note string
		var next []mcp.Server
		next, note, err = fn(old)
		if err != nil {
			return
		}
		a.st.Servers = next
		if perr := a.store.SaveServers(next); perr != nil {
			a.notifyErr(perr.Error())
		}
		a.reconcileMCP(old)
		a.inRT(m.sid, func() { a.notify(note) })
	})
	if !ran {
		return errors.New("meno is closing")
	}
	return err
}

// launch starts s and waits for the result. The server process outlives ctx.
func (m mcpAdmin) launch(ctx context.Context, s mcp.Server) map[string]any {
	a := m.a
	done := make(chan error, 1)
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		err := a.mcp.Start(a.mcpCtx, s)
		done <- err
		a.postEvent(mcpStarted{s.Name, err})
	}()
	select {
	case err := <-done:
		if err != nil {
			res := fail("%s", err)
			res["saved"] = true
			return res
		}
		return m.started(s.Name)
	case <-ctx.Done():
		return fail("gave up waiting for %s to start (the server keeps starting in the background; check mcpServers.list())", s.Name)
	case <-time.After(startWait):
		return fail("%s did not start within %s (it keeps starting in the background; check mcpServers.list())", s.Name, startWait)
	}
}

func (m mcpAdmin) started(name string) map[string]any {
	tools := []any{}
	for _, t := range m.a.mcp.ToolsOf(name) {
		tools = append(tools, map[string]any{"name": t.Name, "description": t.Description})
	}
	return map[string]any{"ok": true, "tools": tools}
}
