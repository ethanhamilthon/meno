package prompt

const bashDescription = `Run one shell command in a fresh shell in the session working directory. Shell state and directory changes do not persist between calls.

Use the meno CLI to interact with Meno:
- meno mcp: servers, tools SERVER [TOOL] (with TOOL: its full description and input schema), call SERVER TOOL [ARGS], results ID, add/remove/restart. Restart reconnects a configured server without a question; add and remove ask the user. To create a new server, edit $MENO_HOME/mcp.json (see meno mcp add --help), then run meno mcp restart NAME.
- meno tasks: start CMD, list, status ID, logs ID, wait ID, cancel ID.
- meno ask FILE|-: ask the user a question using a form JSON file or stdin. Form: {"questions":[{"id":"q1","text":"...","type":"single|multi|text","options":[{"id":"a","label":"..."}],"recommendation":"optional"}],"timeout":seconds?}. Options are required for single and multi, forbidden for text. Prints {id, selected, text} answers as JSON.
- meno view IMAGE: inspect an image.
- meno models [--provider --refresh]: list models.
- meno -p "PROMPT" [--model PROVIDER/MODEL --cwd DIR --resume ID --continue --no-save --json]: run another agent session and print its final answer. It cannot ask the user. The prompt may be piped on stdin. Use meno tasks start to run it in the background. Never start one without explicit user permission.
- meno doctor [--probe]: check the Meno setup (read-only).
- meno help [TOPIC]: show help.
Use meno --help or meno <group/command> --help for details.

Calls of one response run in parallel, so put dependent steps in one call. A call has no time limit. If the user writes while a call runs, it moves to a background task and you get its task ID.
Results end with [exit N] or [cancelled].`

// BashToolDescription is the description of the shell tool.
func BashToolDescription() string { return bashDescription }
