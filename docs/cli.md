# The command line

`meno` with no arguments opens the terminal UI. Other forms:

| Command | What it does |
| --- | --- |
| `meno daemon run\|start\|stop\|restart [--force]\|status\|logs [-f]` | manage the [daemon](daemon.md) |
| `meno sessions [--cwd DIR]` | list sessions: id, status, name, directory |
| `meno send [-s ID \| --new] [--detach] TEXT` | write to a session and print the answer; `--detach` does not wait. If the session is already running, the message is only queued and the command returns at once |
| `meno tail ID` | print the events of a session as JSON lines |
| `meno doctor [--probe]` | read-only health check with suggested fixes |
| `meno web search QUERY \| fetch URL` | search the web with your chosen provider, or fetch a page as text |
| `meno update` | install the latest GitHub release |
| `meno -v`, `meno -h` | version, usage |

`sessions`, `send` and `tail` start the daemon when it is not running. They let you drive
meno from scripts and from other agents without the TUI.

The groups `mcp`, `tasks`, `ask`, `view` and `models` act on a session
(`MENO_SESSION`), which only an agent call has. See [the agent's tool](tools.md).

Run `meno <group> --help` for help; `--json` gives structured output. Exit codes: 0 success,
1 operation error, 2 invalid syntax, 3 forbidden, 124 wait timed out.

## meno doctor

`meno doctor [--probe] [--json]` checks the setup and prints what is wrong with a
suggested fix. It is read-only: it never logs in, restarts, repairs or edits
anything. It checks the daemon socket, version and protocol, the JSON config
files, the integrity of `meno.db`, providers and MCP servers. It works without a
running daemon. `--probe` also lists the models of each enabled provider (bounded
network calls). Secrets are never printed. Exit codes: 0 healthy, 4 warnings, 1 errors.

## meno web

`meno web search QUERY` and `meno web fetch URL`, see [web](web.md).

## meno acp

`meno acp` serves Meno as an Agent Client Protocol agent on stdin/stdout, see [ACP](acp.md).
