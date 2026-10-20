package prompt

const bashDescription = `Run one shell command in the session working directory. The meno groups are mcp, tasks, ask, view, models and help.
- mcp: servers, tools SERVER, call SERVER TOOL [ARGS], results ID, add/remove/restart.
- tasks: start CMD, list, status ID, logs ID, wait ID, cancel ID.
- ask: ask FILE|-.
- view IMAGE; models: [--provider --refresh]; help: [TOPIC].
Use meno --help or meno <group> --help for details. Use meno mcp for MCP servers and tools, meno tasks start for background work, meno ask when a decision is the user's, meno view to inspect an image.
Each call is a fresh shell in the session working directory with no shared state. Use relative paths; do not prefix commands with cd to that directory. Use cd only when a command needs a different directory. Directory changes do not persist between calls; use files for state.
Read skills with cat. python3 and jq may be missing; check with command -v. Use a quoted heredoc for multi-line file edits.
Calls of one response run in parallel, so put dependent steps in one call. A call has no time limit. If the user writes while a call runs, it moves to a background task and you get its task ID.
Results end with [exit N] or [cancelled].

` + bashRules

// BashToolDescription is the description of the shell tool.
func BashToolDescription() string { return bashDescription }
