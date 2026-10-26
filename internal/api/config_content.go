package api

// ConfigSkill describes an installed skill and its enabled state.
type ConfigSkill struct {
	Name     string   `json:"name"`
	Scope    string   `json:"scope"`
	Desc     string   `json:"desc"`
	Path     string   `json:"path"`
	Enabled  bool     `json:"enabled"`
	Auto     bool     `json:"auto"`
	Warnings []string `json:"warnings,omitempty"`
}
type SkillsResponse struct {
	Skills []ConfigSkill `json:"skills"`
}
type SkillResponse struct {
	Skill ConfigSkill `json:"skill"`
}

// SkillEnabledRequest sets the state of a skill: whether it is enabled and,
// when Auto is given, whether it loads into every new session. Cwd is the
// directory the skill is looked up from (project skills depend on it).
type SkillEnabledRequest struct {
	Enabled bool   `json:"enabled"`
	Auto    *bool  `json:"auto,omitempty"`
	Cwd     string `json:"cwd,omitempty"`
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
	Protocol  string            `json:"protocol,omitempty"`
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
