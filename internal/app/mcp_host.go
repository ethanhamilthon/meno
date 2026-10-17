package app

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"meno/internal/config"
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

// agentsPaths returns the global and the project AGENTS.md locations.
func agentsPaths(p config.Paths) (global, project string) {
	return filepath.Join(p.Home, "AGENTS.md"), filepath.Join(p.Cwd, "AGENTS.md")
}

// promptInput gathers everything the system prompt depends on. Unreadable
// AGENTS.md files are skipped and reported in warnings.
func (a *App) promptInput() (in prompt.Input, warnings []string) {
	in.Cwd = a.paths.Cwd
	in.Now = time.Now()
	in.OS = prompt.DescribeOS()
	in.Shell = "/bin/sh"
	for _, s := range a.st.Skills {
		if s.Enabled {
			in.Skills = append(in.Skills, prompt.Skill{Name: s.Name, Desc: s.Desc, Path: s.Path})
		}
	}
	for _, name := range a.mcp.Names() {
		for _, t := range a.mcp.ToolsOf(name) {
			in.Tools = append(in.Tools, prompt.Tool{Server: name, Name: t.Name, Desc: t.Description, Params: t.Signature()})
		}
	}
	g, p := agentsPaths(a.paths)
	var err error
	if in.GlobalAgents, err = prompt.LoadInstructions(g); err != nil {
		warnings = append(warnings, "cannot read "+g+": "+err.Error())
	}
	if in.ProjectAgents, err = prompt.LoadInstructions(p); err != nil {
		warnings = append(warnings, "cannot read "+p+": "+err.Error())
	}
	return in, warnings
}

// systemPrompt builds the system prompt for the current state.
func (a *App) systemPrompt() string {
	in, warnings := a.promptInput()
	for _, w := range warnings {
		a.notifyErr(w)
	}
	return prompt.Build(in)
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
