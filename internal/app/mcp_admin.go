package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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
type mcpAdmin struct{ a *App }

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
		a.notify(note)
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

func (m mcpAdmin) Add(ctx context.Context, spec map[string]any) map[string]any {
	s := mcp.Server{Enabled: true}
	if err := applySpec(&s, spec); err != nil {
		return fail("%s", err)
	}
	if s.Transport == "" {
		switch {
		case s.Command != "" && s.URL == "":
			s.Transport = mcp.TransportStdio
		case s.URL != "" && s.Command == "":
			s.Transport = mcp.TransportHTTP
		}
	}
	if err := mcp.ValidateServer(s); err != nil {
		return fail("%s", err)
	}
	err := m.change(func(sv []mcp.Server) ([]mcp.Server, string, error) {
		for _, o := range sv {
			if o.Name == s.Name {
				return nil, "", fmt.Errorf("mcp server %s already exists; use mcpServers.update", s.Name)
			}
		}
		return append(sv, s), fmt.Sprintf("mcp: agent added %q (%s)", s.Name, s.Transport), nil
	})
	if err != nil {
		return fail("%s", err)
	}
	if !s.Enabled {
		return map[string]any{"ok": true, "tools": []any{}}
	}
	return m.launch(ctx, s)
}

func (m mcpAdmin) Update(ctx context.Context, name string, patch map[string]any) map[string]any {
	var s mcp.Server
	err := m.change(func(sv []mcp.Server) ([]mcp.Server, string, error) {
		i := -1
		for j, o := range sv {
			if o.Name == name {
				i = j
			}
		}
		if i < 0 {
			return nil, "", fmt.Errorf("no mcp server named %s", name)
		}
		s = sv[i]
		if err := applySpec(&s, patch); err != nil {
			return nil, "", err
		}
		if err := mcp.ValidateServer(s); err != nil {
			return nil, "", err
		}
		for j, o := range sv {
			if j != i && o.Name == s.Name {
				return nil, "", fmt.Errorf("mcp server %s already exists", s.Name)
			}
		}
		if s.Name != name {
			m.a.mcp.Stop(name)
		}
		sv[i] = s
		return sv, fmt.Sprintf("mcp: agent updated %q", s.Name), nil
	})
	if err != nil {
		return fail("%s", err)
	}
	if !s.Enabled {
		m.a.mcp.Stop(s.Name)
		m.a.postEvent(mcpStarted{s.Name, nil})
		return map[string]any{"ok": true, "tools": []any{}}
	}
	return m.launch(ctx, s)
}

func (m mcpAdmin) Remove(name string) map[string]any {
	err := m.change(func(sv []mcp.Server) ([]mcp.Server, string, error) {
		for i, o := range sv {
			if o.Name == name {
				m.a.mcp.Stop(name)
				return append(sv[:i], sv[i+1:]...), fmt.Sprintf("mcp: agent removed %q", name), nil
			}
		}
		return nil, "", fmt.Errorf("no mcp server named %s", name)
	})
	if err != nil {
		return fail("%s", err)
	}
	return map[string]any{"ok": true}
}

func (m mcpAdmin) Restart(ctx context.Context, name string) map[string]any {
	var s mcp.Server
	var found bool
	m.a.onUI(func() {
		if i := m.find(name); i >= 0 {
			s, found = m.a.st.Servers[i], true
		}
	})
	if !found {
		return fail("no mcp server named %s", name)
	}
	if !s.Enabled {
		return fail("%s is disabled; enable it with mcpServers.update(%q, {enabled: true})", name, name)
	}
	return m.launch(ctx, s)
}

// reconcileMCP keeps the open :mcp dialogs valid after the server list changed
// behind them. old is the list before the change.
func (a *App) reconcileMCP(old []mcp.Server) {
	if n := len(a.st.Servers); a.st.Sel > n {
		a.st.Sel = n
	}
	if a.st.Over != OverMCPEdit || a.st.EditIdx < 0 || a.st.EditIdx >= len(old) {
		return
	}
	name := old[a.st.EditIdx].Name
	for i, s := range a.st.Servers {
		if s.Name == name {
			a.st.EditIdx = i
			return
		}
	}
	a.openOverlay(OverMCP)
	a.notifyErr("mcp server " + strings.TrimSpace(name) + " was removed or renamed; edit closed")
}
