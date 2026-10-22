package engine

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/mcp"
)

func (e *Engine) mcpResource(ctx context.Context, s *runState, tail []string, method string, data map[string]any, query string) (any, error) {
	if len(tail) == 0 {
		return nil, dapi(api.CodeBadRequest, "MCP resource operation is required")
	}
	op := tail[0]
	if op == "servers" {
		out := []map[string]any{}
		servers, err := e.mcpServers(ctx)
		if err != nil {
			return nil, err
		}
		for _, srv := range servers {
			status := e.mcp.Snapshot(srv.Name)
			out = append(out, map[string]any{"name": srv.Name, "enabled": srv.Enabled, "running": status.Running, "starting": status.Starting, "error": status.Error})
		}
		return out, nil
	}
	if op == "tools" {
		name := stringValue(data, "server")
		if len(tail) > 1 {
			name = tail[1]
		}
		var out []any
		for _, t := range e.mcp.ToolsOf(name) {
			var schema any
			_ = json.Unmarshal(t.InputSchema, &schema)
			out = append(out, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": schema})
		}
		return out, nil
	}
	if op == "call" {
		server, tool := stringValue(data, "server"), stringValue(data, "tool")
		args, _ := data["args"].(map[string]any)
		return e.callMCP(ctx, s, server, tool, args)
	}
	if op == "results" {
		id := stringValue(data, "id")
		if len(tail) > 1 {
			id = tail[1]
		}
		store := mcp.ResultStore{Dir: e.paths.File("mcp-results")}
		off, lim := int(number(data["offset"])), int(number(data["limit"]))
		if lim == 0 {
			lim = 10000
		}
		return store.Read(s.session.ID, id, off, lim)
	}
	if op == "add" || op == "remove" || op == "restart" {
		return e.adminMCP(ctx, s, op, stringValue(data, "server"))
	}
	return nil, dapi(api.CodeNotFound, "unknown MCP operation")
}
