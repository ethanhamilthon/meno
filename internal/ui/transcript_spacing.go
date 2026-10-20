package ui

// TurnHasMessage reports whether a turn needs a blank line after its content.
// Tool-only turns stay adjacent, including when their output is visible.
func TurnHasMessage(t Turn, js JSOpts) bool {
	return t.Role == RoleUser || t.Role == RoleAssistant && (t.Text != "" || js.ThinkMode != JSOff && t.Thinking != "")
}
