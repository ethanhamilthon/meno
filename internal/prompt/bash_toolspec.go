package prompt

const bashDescription = `Run one shell command in a fresh shell in the session working directory. Shell state and directory changes do not persist between calls.

Use the meno CLI to interact with Meno:
- meno mcp: servers, tools SERVER [TOOL] (with TOOL: its full description and input schema), call SERVER TOOL [ARGS], results ID, add/remove/restart. Restart reconnects a configured server without a question; add and remove ask the user. To create a new server, edit $MENO_HOME/mcp.json (see meno mcp add --help), then run meno mcp restart NAME.
- meno tasks: start CMD, list, status ID, logs ID, wait ID, cancel ID.
- meno ask FILE|-: ask the user a question using a form JSON file or stdin. Form: {"questions":[{"id":"q1","text":"...","type":"single|multi|text","options":[{"id":"a","label":"..."}],"recommendation":"optional"}],"timeout":seconds?}. Options are required for single and multi, forbidden for text. Prints {id, selected, text} answers as JSON.
- meno view IMAGE: inspect an image.
- meno models [--provider --refresh]: list models.
- meno sessions [--cwd DIR --status idle|running|needs-you --spawned-by ID|me --title-prefix P]: list sessions. meno sessions show ID gives the state, the open question and the latest answer; cancel ID stops its run; answer ID QUESTION ANSWERS_JSON|--cancel replies to its question.
- meno send (-s ID|--new|--fork SESSION) [--at N --model PROVIDER/MODEL --effort E --cwd DIR --title T --system NAME --no-ask --timeout S --detach --keep-running] TEXT: write to an existing session, a new one, or a fork of SESSION, and print the answer when the run ends (to work in the background wrap it in meno tasks start; --json gives the session id and status). If that session is already running the message is only queued and the command returns at once. Exit 5 means the session waits for an answer to a question. Never create new sessions or send messages to other sessions without explicit user permission.
- meno tail ID... [--children-of ID|me --events T,...]: stream session events as JSON lines.
- meno doctor [--probe]: check the Meno setup (read-only).
- meno help [TOPIC]: show help.
Use meno --help or meno <group/command> --help for details.

Calls of one response run in parallel, so put dependent steps in one call. A call has no time limit. If the user writes while a call runs, it moves to a background task and you get its task ID.
Results end with [exit N] or [cancelled].`

// BashToolDescription is the description of the shell tool.
func BashToolDescription() string { return bashDescription }
