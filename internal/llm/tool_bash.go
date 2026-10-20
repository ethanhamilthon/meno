package llm

// BashToolName is the name of the shell tool.
const BashToolName = "bash"

// toolSchema is the definition of the tool a request offers.
func (r Request) toolSchema() map[string]any {
	if r.ToolName == BashToolName {
		return BashSchema(r.ToolDescription)
	}
	return ToolSchema(r.ToolDescription)
}

// BashSchema is the definition of the bash tool: a command. An empty desc gives a short default.
func BashSchema(desc string) map[string]any {
	if desc == "" {
		desc = "Run a shell command in the session working directory."
	}
	props := map[string]any{
		"command": map[string]any{"type": "string", "description": "Shell command to run."},
	}
	required := []string{"command"}
	return map[string]any{
		"type":        "function",
		"name":        BashToolName,
		"description": desc,
		"parameters": map[string]any{
			"type":       "object",
			"properties": props,
			"required":   required,
		},
	}
}
