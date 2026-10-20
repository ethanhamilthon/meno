package prompt

const bashDescription = `Run one shell command in the session working directory. The meno groups are mcp, tasks, ask, view, models and help.
- mcp: servers, tools SERVER, call SERVER TOOL [ARGS], results ID, add/remove/restart.
- tasks: start CMD, list, status ID, logs ID, wait ID, cancel ID.
- ask: ask FILE|-.
- view IMAGE; models: [--provider --refresh]; help: [TOPIC].
Use meno --help or meno <group> --help for details. Read skills with cat.
python3 and jq may be missing; check with command -v. Use a quoted heredoc for multi-line edits. Find with rg (check command -v rg) or grep -rn.
Each call is a fresh shell with no shared state; use files or meno tasks start for background work.
Calls of one response run in parallel, so put dependent steps in one call. A call has no time limit. If the user writes while a call runs, it moves to a background task and you get its task ID.
Results end with [exit N] or [cancelled].`

// BashToolDescriptionFor returns the unrestricted shell tool description.
func BashToolDescriptionFor(_ []string) string { return bashDescription }
