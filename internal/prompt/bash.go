package prompt

import "strings"

// Build returns the full system prompt: the identity line and the data of the
// session. Sections go from the most stable to the most volatile so that a
// change only invalidates the text after it. How to use bash lives in the tool
// description (BashToolDescription).
func Build(in Input) string {
	var parts []string
	add := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	if id := strings.TrimSpace(in.Identity); id != "" {
		add(id)
	} else {
		add(identityPrompt)
	}
	add(skillsSection(in.Skills))
	add(instructionsSection("project-instructions", in.ProjectAgents))
	add(mcpToolsSection(in.Tools))
	add(environmentSection(in))
	return strings.Join(parts, "\n\n")
}
