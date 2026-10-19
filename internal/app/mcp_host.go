package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"
	"unicode/utf8"

	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/mcp"
	"meno/internal/safe"
)

// mcpHost lets the js VM call the tools of the running MCP servers. Complete
// results are stored per session before the VM sees a bounded projection.
type mcpHost struct {
	m       *mcp.Manager
	results mcp.ResultStore
	sid     string
}

func (h mcpHost) Tools() []jsvm.MCPTool {
	var out []jsvm.MCPTool
	for _, name := range h.m.Names() {
		for _, t := range h.m.ToolsOf(name) {
			out = append(out, jsvm.MCPTool{Server: name, Name: t.Name, Description: t.Description, Schema: t.InputSchema})
		}
	}
	return out
}

// projectionBytes bounds each part of a result shown to the model.
const projectionBytes = 32 << 10

func (h mcpHost) Call(ctx context.Context, server, tool string, args map[string]any) (jsvm.MCPResult, error) {
	c, ok := h.m.Client(server)
	if !ok {
		return jsvm.MCPResult{}, errors.New("server " + server + " is not running")
	}
	res, err := c.CallResult(ctx, tool, args)
	if err != nil {
		return jsvm.MCPResult{}, err
	}
	out := jsvm.MCPResult{Text: res.Text, Structured: res.Structured, Content: res.Content, IsError: res.IsError}
	st, serr := h.results.Save(h.sid, server, tool, res)
	out.ResultID = st.ID
	out.Notes = st.Notes
	if serr != nil {
		out.ResultID = ""
		out.Notes = append(out.Notes, "result could not be stored locally: "+serr.Error())
		out.Truncated = true
	}
	for _, a := range st.Artifacts {
		out.Artifacts = append(out.Artifacts, map[string]any{"index": a.Index, "kind": a.Kind, "mime": a.Mime, "size": a.Size, "path": a.Path, "uri": a.URI})
	}
	if len(out.Text) > projectionBytes {
		out.Text, out.Truncated = cutText(out.Text, projectionBytes), true
	}
	if len(out.Structured) > projectionBytes {
		out.Structured, out.Truncated = nil, true
		out.Notes = append(out.Notes, "structuredContent is too large to show; read it with mcpResults.read")
	}
	out.Content = boundContent(out.Content, &out.Truncated)
	if out.Truncated && out.ResultID != "" {
		out.Notes = append(out.Notes, "read the complete result with mcpResults.read('"+out.ResultID+"', {offset})")
	}
	return out, nil
}

// boundContent drops binary payloads (they are artifacts) and cuts long text.
func boundContent(blocks []json.RawMessage, truncated *bool) []json.RawMessage {
	out := make([]json.RawMessage, len(blocks))
	budget := projectionBytes
	for i, raw := range blocks {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			out[i] = raw
			continue
		}
		for _, holder := range []map[string]any{m, nested(m, "resource")} {
			if holder == nil {
				continue
			}
			for _, k := range []string{"data", "blob"} {
				if s, ok := holder[k].(string); ok && len(s) > 0 {
					delete(holder, k)
					holder["artifact"] = "see artifacts"
				}
			}
			if s, ok := holder["text"].(string); ok {
				if len(s) > budget {
					holder["text"], *truncated = cutText(s, max(budget, 0)), true
				}
				budget -= min(len(s), budget)
			}
		}
		out[i], _ = json.Marshal(m)
	}
	return out
}

func nested(m map[string]any, k string) map[string]any {
	n, _ := m[k].(map[string]any)
	return n
}

func cutText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func (h mcpHost) ResultInfo(id string) (any, error) {
	st, err := h.results.Info(h.sid, id)
	if err != nil {
		return nil, err
	}
	return toAny(st)
}

func (h mcpHost) ResultRead(id string, offset, limit int) (any, error) {
	sl, err := h.results.Read(h.sid, id, offset, limit)
	if err != nil {
		return nil, err
	}
	return toAny(sl)
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
	a.reloadModes() // modes live next to the skills: the project ones follow the directory
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
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		err := a.mcp.Start(a.mcpCtx, s)
		a.mcp.EndStart(s.Name)
		a.postEvent(mcpStarted{s.Name, err})
	}()
}
