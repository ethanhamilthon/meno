# The command line

`meno` with no arguments opens the terminal UI. Other forms:

| Command | What it does |
| --- | --- |
| `meno daemon run\|start\|stop\|restart [--force]\|status\|logs [-f]` | manage the [daemon](daemon.md) |
| `meno sessions [--cwd DIR]` | list sessions: id, status, name, directory |
| `meno send [-s ID \| --new] [--detach] TEXT` | write to a session and print the answer; `--detach` does not wait |
| `meno tail ID` | print the events of a session as JSON lines |
| `meno update` | install the latest GitHub release |
| `meno -v`, `meno -h` | version, usage |

`sessions`, `send` and `tail` start the daemon when it is not running. They let you drive
meno from scripts and from other agents without the TUI.

The groups `mcp`, `tasks`, `ask`, `view` and `models` act on a session
(`MENO_SESSION`), which only an agent call has. See [the agent's tool](tools.md).

Run `meno <group> --help` for help; `--json` gives structured output. Exit codes: 0 success,
1 operation error, 2 invalid syntax, 3 forbidden, 124 wait timed out.
