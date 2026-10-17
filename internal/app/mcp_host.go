package app

import (
	"context"
	"errors"

	"meno/internal/jsvm"
	"meno/internal/mcp"
	"meno/internal/prompt"
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

// systemPrompt is the base prompt plus the enabled skills and the tools of the
// MCP servers that are running now.
func (a *App) systemPrompt() string {
	var skills []prompt.Skill
	for _, s := range a.st.Skills {
		if s.Enabled {
			skills = append(skills, prompt.Skill{Name: s.Name, Desc: s.Desc, Path: s.Path})
		}
	}
	var tools []prompt.Tool
	for _, name := range a.mcp.Names() {
		for _, t := range a.mcp.ToolsOf(name) {
			tools = append(tools, prompt.Tool{Server: name, Name: t.Name, Desc: t.Description, Params: t.Signature()})
		}
	}
	return prompt.Build(a.paths.Cwd, skills, tools)
}

// reloadSkills picks up skill files added, edited or removed since the last look.
func (a *App) reloadSkills() {
	if sk, err := a.store.LoadSkills(); err == nil {
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
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		a.postEvent(mcpStarted{s.Name, a.mcp.Start(a.mcpCtx, s)})
	}()
}
