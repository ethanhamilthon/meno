package app

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/mcp"
)

// mcpHost lets the js VM call the tools of the running MCP servers.
type mcpHost struct{ m *mcp.Manager }

func (h mcpHost) Tools() []jsvm.MCPTool {
	var out []jsvm.MCPTool
	for _, name := range h.m.Names() {
		for _, t := range h.m.ToolsOf(name) {
			out = append(out, jsvm.MCPTool{Server: name, Name: t.Name, Description: t.Description, Schema: t.InputSchema})
		}
	}
	return out
}

func (h mcpHost) Call(ctx context.Context, server, tool string, args map[string]any) (string, error) {
	c, ok := h.m.Client(server)
	if !ok {
		return "", errors.New("server " + server + " is not running")
	}
	return c.Call(ctx, tool, args)
}

// agentsPaths returns the global and the project AGENTS.md locations.
func agentsPaths(p config.Paths) (global, project string) {
	return filepath.Join(p.Home, "AGENTS.md"), filepath.Join(p.Cwd, "AGENTS.md")
}

// worldSrc captures what a prompt build needs. Call it on the UI goroutine;
// the result may be used from any goroutine.
func (a *App) modelSig() modelSig {
	p, m, e := a.selection()
	return modelSig{p, m, e}
}

func (a *App) worldSrc() worldSrc {
	paths := a.paths
	paths.Cwd = a.st.Cwd // the current session's directory
	return worldSrc{
		Paths: paths, MCP: a.mcp, Servers: append([]mcp.Server(nil), a.st.Servers...), Now: time.Now(),
		Model: a.modelSig(),
	}
}

// reloadSkills picks up skill files added, edited or removed since the last look.
func (a *App) reloadSkills() {
	p := a.paths
	p.Cwd = a.st.Cwd
	if sk, err := config.NewStore(p).LoadSkills(); err == nil {
		a.st.Skills = sk
	}
}

// startServers starts every enabled MCP server in the background; failures are
// reported in the timeline when they happen.
func (a *App) startServers() {
	for i := range a.st.Servers {
		if a.st.Servers[i].Enabled {
			a.startMCPAsync(a.st.Servers[i])
		}
	}
}

type mcpStarted struct {
	name string
	err  error
}

func (a *App) startMCPAsync(s mcp.Server) {
	// Marked before the goroutine runs so that a prompt build waits for it.
	a.mcp.BeginStart(s.Name)
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		err := a.mcp.Start(a.mcpCtx, s)
		a.mcp.EndStart(s.Name)
		a.postEvent(mcpStarted{s.Name, err})
	}()
}
