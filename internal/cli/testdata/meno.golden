# meno

Use `meno <command>`. Run `meno <command> --help` for details; `meno help TOPIC` also works.

- mcp: use the user's MCP servers: servers, tools, call, results, add, remove, restart.
- tasks: run a command in the background and follow it: start, list, status, logs, wait, cancel.
- ask: put a question with options to the user and get the answer as JSON.
- view: show an image file to yourself.
- models: list the models of the enabled providers.
- meno -p "PROMPT": run another agent session and print its final answer. Options: --model PROVIDER/MODEL, --cwd DIR,
  --resume ID, --continue, --no-save, --json. It cannot ask the user. The prompt may be piped on stdin. Ask the user before
  you start one; run it with `meno tasks start` to work meanwhile.
- doctor: check the setup (read-only). It names what is wrong and how to fix it.
- update, acp: for people; not for agents.

Arguments of API commands are JSON objects or key=value pairs. Add --json for structured output.
Configuration lives in $MENO_HOME (set in every call; ~/.meno by default): settings.json,
provider.json, mcp.json. They hold secrets: do not print them whole.

Exit codes: 0 success, 1 error, 2 usage, 3 forbidden, 4 doctor found warnings,
124 a wait timed out.
