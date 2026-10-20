package cli

var commandNames = []string{"read", "edit", "write", "search", "files", "mcp", "tasks", "ask", "goal", "web", "view", "models", "daemon", "sessions", "send", "tail"}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
