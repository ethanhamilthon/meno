package prompt

import (
	"strings"

	"meno/internal/modes"
)

const bashDescription = `Run one shell command in the session working directory. The meno groups are mcp, tasks, ask, goal, view, models and help.
- mcp: servers, tools SERVER, call SERVER TOOL [ARGS], results ID, add/remove/restart.
- tasks: start CMD, list, status ID, logs ID, wait ID, cancel ID.
- ask: ask FILE|-; goal: done SUMMARY|blocked REASON.
- view IMAGE; models: [--provider --refresh]; help: [TOPIC].
Use meno --help or meno <group> --help for details. Read skills with cat.
python3 and jq may be missing; check with command -v. Use a quoted heredoc for multi-line edits. Find with rg (check command -v rg) or grep -rn.
Each call is a fresh shell with no shared state; use files or meno tasks start for background work.
Results end with [exit N], [timed out after Ns] or [cancelled].`

// BashToolDescriptionFor lists allowed groups and documents per-call timeouts when enabled.
func BashToolDescriptionFor(allowed []string, modelTimeout bool) string {
	return bashToolDescription(allowed, modelTimeout)
}

func bashToolDescription(allowed []string, modelTimeout bool) string {
	text := bashDescription
	if modelTimeout {
		text += "\nThe optional timeout argument is in seconds."
	}
	if allowed == nil {
		return text
	}
	var lines []string
	for _, group := range []string{modes.GroupMCP, modes.GroupTasks, modes.GroupAsk, modes.GroupGoal, modes.GroupView, modes.GroupModels, modes.GroupHelp} {
		if modes.Allows(allowed, group) {
			lines = append(lines, group)
		}
	}
	return text + "\n\nRestricted mode. Allowed groups: " + strings.Join(lines, ", ") + ". Other groups are forbidden."
}
