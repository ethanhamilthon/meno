package cli

func helpTopic(group string, before []string) string {
	if len(before) == 0 {
		return group
	}
	switch group {
	case "mcp", "tasks":
		return group + "-" + before[0]
	default:
		return group
	}
}
