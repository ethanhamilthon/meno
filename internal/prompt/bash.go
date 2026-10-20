package prompt

import "strings"

func buildBash(in Input) string {
	var parts []string
	add := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	add(identityPrompt)
	add(voiceSection())
	add(bashMethod(in.Allow))
	add(bashEditingSection())
	add(verificationSection())
	add(outputSection())
	add(bashGuidelines(in.Allow))
	add(skillsSection(in.Skills))
	add(instructionsSection("global-instructions", in.GlobalAgents))
	add(instructionsSection("project-instructions", in.ProjectAgents))
	add(mcpToolsSection(in.Tools))
	add(environmentSection(in))
	return strings.Join(parts, "\n\n")
}

func allows(groups []string, group string) bool {
	if groups == nil {
		return true
	}
	for _, item := range groups {
		if item == group {
			return true
		}
	}
	return false
}

func bashMethod(groups []string) string {
	var lines []string
	if allows(groups, "mcp") {
		lines = append(lines, "- Use meno mcp for MCP servers and tools.")
	}
	if allows(groups, "tasks") {
		lines = append(lines, "- Use meno tasks start for background work; check and stop tasks by id.")
	}
	if allows(groups, "ask") {
		lines = append(lines, "- Use meno ask when you need a user decision.")
	}
	if allows(groups, "view") {
		lines = append(lines, "- Use meno view to inspect an image.")
	}
	if allows(groups, "models") {
		lines = append(lines, "- Use meno models to inspect or refresh model choices.")
	}
	if allows(groups, "help") {
		lines = append(lines, "- Use meno --help or meno <group> --help for command details.")
	}
	return element("method", strings.Join(lines, "\n"))
}

func bashEditingSection() string {
	return element("editing", "- Read a file before you change it. Change part of a file with sed or a quoted heredoc, not by sending the whole file again. Do not overwrite content you have not read.\n- Find with rg when it exists (check with command -v rg), otherwise grep -rn. Never guess a path or a symbol.")
}

func bashGuidelines(groups []string) string {
	first := "- Your only tool is bash. Each Bash call starts a fresh shell in the working directory shown in the environment section. Use relative paths; do not prefix commands with cd to that directory. Use cd only when a command needs a different directory. Directory changes do not persist between calls; use files for state."
	if allows(groups, "tasks") {
		first += " Use meno tasks start for background work."
	}
	lines := []string{first, "- Read skills with cat. python3 and jq may be missing; check with command -v.", "- Use a quoted heredoc for multi-line file edits.", "- Results end with [exit N] or [cancelled]. Calls of one response run in parallel; put dependent steps in one call. A call has no time limit; if the user writes meanwhile it moves to a background task."}
	if allows(groups, "ask") {
		lines = append(lines, "- Ask the user with meno ask when a decision is theirs to make.")
	}
	return element("guidelines", strings.Join(lines, "\n"))
}
