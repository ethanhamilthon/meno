package engine

import (
	"context"
	"encoding/json"
	"meno/internal/mcp"
	"meno/internal/proto"
)

func (e *Engine) mcpServers(ctx context.Context) ([]mcp.Server, error) {
	if e.resources == nil {
		return e.serverList(), nil
	}
	r, err := e.resources.Resource(ctx, "GET", "/v1/mcp/servers", "", proto.ResourceRequest{})
	if err != nil {
		return nil, err
	}
	var out proto.McpServersResponse
	if err = json.Unmarshal(r.Data, &out); err != nil {
		return nil, err
	}
	servers := make([]mcp.Server, 0, len(out.Servers))
	for _, v := range out.Servers {
		servers = append(servers, mcp.Server{Name: v.Name, Transport: v.Transport, Command: v.Command, Env: v.Env, URL: v.URL, Headers: v.Headers, Enabled: v.Enabled, Source: v.Source})
	}
	return servers, nil
}
