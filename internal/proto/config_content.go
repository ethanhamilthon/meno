package proto

// ConfigSkill describes an installed skill and its enabled state.
type ConfigSkill struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Desc    string `json:"desc"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}
type SkillsResponse struct {
	Skills []ConfigSkill `json:"skills"`
}
type SkillResponse struct {
	Skill ConfigSkill `json:"skill"`
}
type SkillEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// ConfigMCPServer describes one configured MCP server.
type ConfigMCPServer struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Enabled   bool              `json:"enabled"`
	Source    string            `json:"source"`
	Running   bool              `json:"running"`
	Starting  bool              `json:"starting"`
	Error     string            `json:"error,omitempty"`
}
type McpServersResponse struct {
	Servers  []ConfigMCPServer `json:"servers"`
	Warnings []string          `json:"warnings,omitempty"`
}
type McpServerRequest struct {
	Server ConfigMCPServer `json:"server"`
}
type McpServerResponse struct {
	Server ConfigMCPServer `json:"server"`
}
