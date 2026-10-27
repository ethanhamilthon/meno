# The command line

`meno` with no arguments opens the terminal UI. Other forms:

| Command | What it does |
| --- | --- |
| `meno resume [ID]` | open the UI on a stored session; without an ID the list of sessions opens |
| `meno --resume ID`, `meno --continue` | the same, or the newest stored session of the current directory |
| `meno -p [PROMPT]` | run one prompt without the UI and print the final answer; see below |
| `meno -p --stdio` | serve [JSON-RPC over stdio](stdio.md) |
| `meno doctor [--probe]` | read-only health check with suggested fixes |
| `meno update` | install the latest GitHub release |
| `meno -v`, `meno -h` | version, usage |

The groups `mcp`, `tasks`, `ask`, `view` and `models` are for the agent: they act on the session
that runs the call (`MENO_SESSION`), see [the agent's tool](tools.md). `mcp`, `models` and `doctor`
also work in your shell, on an engine started for that one command; `tasks`, `ask` and `view` need an
agent call.

Run `meno <group> --help` for help; `--json` gives structured output. Exit codes: 0 success,
1 operation error, 2 invalid syntax, 3 forbidden, 124 wait timed out (`meno doctor` also uses 4 for
warnings).

## No daemon

Since 0.8.0 nothing runs in the background. Each meno window, `meno -p` run, `meno acp` connection and
`meno -p --stdio` server runs its own engine; when it ends, its runs and background tasks end too.
Several processes may use one `MENO_HOME`; they share the stored sessions. An agent call reaches the
engine that runs it through a descriptor the call inherits (`MENO_FD`), not through a socket file.

## meno -p

`meno -p "PROMPT"` creates a session in the current directory, sends the prompt, waits for the run and
prints the final answer on stdout (errors on stderr, exit 1 on a failed run). The prompt may also be
piped on stdin. Options:

- `--model PROVIDER/MODEL` chooses the model; `--cwd DIR` the directory.
- `--resume ID` continues a stored session, `--continue` the newest one of the directory.
- `--no-save` deletes the session when the run ends (not allowed with `--resume` or `--continue`). Without it the session is stored and can be resumed.
- A resumed session keeps the `--no-ask` setting it was created with.
- `--json` prints `{session_id, text, status, error}` instead of the text.
- `--no-ask` is implied when stdin is not a terminal: a question of the agent fails the run.

An agent starts another agent by running `meno -p` in a bash call, usually inside `meno tasks start` so it
can work meanwhile. Tell the agent not to do this without your permission if you do not want it.

## meno doctor

`meno doctor [--probe] [--json]` checks the setup and prints what is wrong with a
suggested fix. It is read-only: it never logs in, restarts, repairs or edits
anything. It checks the JSON config files, the integrity of `meno.db`, providers and
MCP servers. `--probe` also lists the models of each enabled provider (bounded
network calls). Secrets are never printed. Exit codes: 0 healthy, 4 warnings, 1 errors.

## meno acp

`meno acp` serves Meno as an Agent Client Protocol agent on stdin/stdout, see [ACP](acp.md). `--no-ask` creates sessions without `meno ask`.

## --no-ask

`meno --no-ask`, `meno -p --no-ask` and `meno acp --no-ask` create sessions in which `meno ask` is turned off: the tool answers with an error (exit 3) that tells the agent to write its question in the chat. The setting belongs to the session: it is stored and a fork keeps it. It cannot be turned off later. A session you open with `/resume` keeps the setting it was created with.
