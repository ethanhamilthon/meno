package client

import (
	"context"
	"net/url"

	"meno/internal/api"
)

// Skills lists installed skills and changes their enabled state.
func (c *Client) Skills(ctx context.Context) (api.SkillsResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/skills", nil)
	if e != nil {
		return api.SkillsResponse{}, e
	}
	return configDecode[api.SkillsResponse](raw)
}
func (c *Client) SetSkillEnabled(ctx context.Context, name string, enabled bool) (api.ConfigSkill, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/skills/"+pathID(name)+"/enabled", api.SkillEnabledRequest{Enabled: enabled})
	if e != nil {
		return api.ConfigSkill{}, e
	}
	r, e := configDecode[api.SkillResponse](raw)
	return r.Skill, e
}

// McpServers lists and manages daemon MCP server configurations.
func (c *Client) McpServers(ctx context.Context) (api.McpServersResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/mcp/servers", nil)
	if e != nil {
		return api.McpServersResponse{}, e
	}
	return configDecode[api.McpServersResponse](raw)
}
func (c *Client) AddMcpServer(ctx context.Context, v api.ConfigMCPServer) (api.McpServerResponse, error) {
	raw, e := c.configCall(ctx, "POST", "/v1/mcp/servers", api.McpServerRequest{Server: v})
	if e != nil {
		return api.McpServerResponse{}, e
	}
	return configDecode[api.McpServerResponse](raw)
}
func (c *Client) UpdateMcpServer(ctx context.Context, name string, v api.ConfigMCPServer) (api.McpServerResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/mcp/servers/"+pathID(name), api.McpServerRequest{Server: v})
	if e != nil {
		return api.McpServerResponse{}, e
	}
	return configDecode[api.McpServerResponse](raw)
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
func (c *Client) Models(ctx context.Context, options api.ModelsRequest) (api.ModelsResponse, error) {
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
		return api.ModelsResponse{}, e
	}
	return configDecode[api.ModelsResponse](raw)
}
func (c *Client) FileSearch(ctx context.Context, root, query string, limit int) (api.FileSearchResponse, error) {
	return c.SearchFiles(ctx, api.FileSearchRequest{Root: root, Query: query, Limit: limit})
}
