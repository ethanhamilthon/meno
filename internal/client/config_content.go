package client

import (
	"context"
	"net/url"

	"meno/internal/proto"
)

// Skills lists installed skills and changes their enabled state.
func (c *Client) Skills(ctx context.Context) (proto.SkillsResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/skills", nil)
	if e != nil {
		return proto.SkillsResponse{}, e
	}
	return configDecode[proto.SkillsResponse](raw)
}
func (c *Client) SetSkillEnabled(ctx context.Context, name string, enabled bool) (proto.ConfigSkill, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/skills/"+pathID(name)+"/enabled", proto.SkillEnabledRequest{Enabled: enabled})
	if e != nil {
		return proto.ConfigSkill{}, e
	}
	r, e := configDecode[proto.SkillResponse](raw)
	return r.Skill, e
}

// McpServers lists and manages daemon MCP server configurations.
func (c *Client) McpServers(ctx context.Context) (proto.McpServersResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/mcp/servers", nil)
	if e != nil {
		return proto.McpServersResponse{}, e
	}
	return configDecode[proto.McpServersResponse](raw)
}
func (c *Client) AddMcpServer(ctx context.Context, v proto.ConfigMCPServer) (proto.McpServerResponse, error) {
	raw, e := c.configCall(ctx, "POST", "/v1/mcp/servers", proto.McpServerRequest{Server: v})
	if e != nil {
		return proto.McpServerResponse{}, e
	}
	return configDecode[proto.McpServerResponse](raw)
}
func (c *Client) UpdateMcpServer(ctx context.Context, name string, v proto.ConfigMCPServer) (proto.McpServerResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/mcp/servers/"+pathID(name), proto.McpServerRequest{Server: v})
	if e != nil {
		return proto.McpServerResponse{}, e
	}
	return configDecode[proto.McpServerResponse](raw)
}
func (c *Client) RemoveMcpServer(ctx context.Context, name string) error {
	_, e := c.configCall(ctx, "DELETE", "/v1/mcp/servers/"+pathID(name), nil)
	return e
}
func (c *Client) RestartMcpServer(ctx context.Context, name string) error {
	_, e := c.configCall(ctx, "POST", "/v1/mcp/servers/"+pathID(name)+"/restart", struct{}{})
	return e
}

// Models lists provider models, catalog pricing, and refresh results.
func (c *Client) Models(ctx context.Context, options proto.ModelsRequest) (proto.ModelsResponse, error) {
	q := url.Values{}
	if options.Provider != "" {
		q.Set("provider", options.Provider)
	}
	if options.Refresh {
		q.Set("refresh", "true")
	}
	if options.RefreshCatalog {
		q.Set("catalog", "refresh")
	}
	raw, e := c.configCall(ctx, "GET", "/v1/models?"+q.Encode(), nil)
	if e != nil {
		return proto.ModelsResponse{}, e
	}
	return configDecode[proto.ModelsResponse](raw)
}
func (c *Client) FileSearch(ctx context.Context, root, query string, limit int) (proto.FileSearchResponse, error) {
	return c.SearchFiles(ctx, proto.FileSearchRequest{Root: root, Query: query, Limit: limit})
}
