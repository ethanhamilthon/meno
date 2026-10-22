package api

// ConfigBundle batches the existing configuration read responses. Components
// are read independently; this is not an atomic snapshot across stores.
type ConfigBundle struct {
	Settings  SettingsResponse   `json:"settings"`
	Providers ProvidersResponse  `json:"providers"`
	MCP       McpServersResponse `json:"mcp"`
	Skills    SkillsResponse     `json:"skills"`
}
