# meno

Use `meno <command>`. Run `meno <command> --help` for details; `meno help TOPIC` also works.

- mcp: use the user's MCP servers: servers, tools, call, results, add, remove, restart.
- tasks: run a command in the background and follow it: start, list, status, logs, wait, cancel.
- ask: put a question with options to the user and get the answer as JSON.
- view: show an image file to yourself.
- models: list the models of the enabled providers.
- sessions, send, tail: list, drive and follow other sessions. Ask the user before you start or message one.
- doctor: check the setup (read-only). It names what is wrong and how to fix it.
- daemon, update, acp: for people; not for agents.

Arguments of API commands are JSON objects or key=value pairs. Add --json for structured output.
Configuration lives in $MENO_HOME (set in every call; ~/.meno by default): settings.json,
provider.json, mcp.json. They hold secrets: do not print them whole.

Exit codes: 0 success, 1 error, 2 usage, 3 forbidden, 4 doctor found warnings,
5 a session waits for an answer, 6 the run was stopped, 124 a wait timed out.
