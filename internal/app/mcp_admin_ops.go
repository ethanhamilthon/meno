package app

import (
	"context"
	"fmt"
	"strings"

	"meno/internal/mcp"
)

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
