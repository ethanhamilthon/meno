package llm

// BashToolName is the name of the shell tool.
const BashToolName = "bash"

// toolSchema is the definition of the tool a request offers.
func (r Request) toolSchema() map[string]any {
	if r.ToolName == BashToolName {
		return BashSchema(r.ToolDescription, r.ModelTimeout)
	}
	return ToolSchema(r.ToolDescription, r.ModelTimeout)
}

// BashSchema is the definition of the bash tool: a command, and a timeout in
// seconds only when the model picks it. An empty desc gives a short default.
func BashSchema(desc string, modelTimeout bool) map[string]any {
	if desc == "" {
		desc = "Run a shell command in the session working directory."
	}
	props := map[string]any{
		"command": map[string]any{"type": "string", "description": "Shell command to run."},
	}
	required := []string{"command"}
	if modelTimeout {
		props["timeout"] = map[string]any{
			"type":        "number",
			"description": "Seconds this call may run before it is stopped (0 means no limit). Raise it for slow commands such as builds or test suites.",
		}
	}
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
