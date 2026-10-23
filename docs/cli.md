# The command line

`meno` with no arguments opens the terminal UI. Other forms:

| Command | What it does |
| --- | --- |
| `meno daemon run\|start\|stop\|restart [--force]\|status\|logs [-f]` | manage the [daemon](daemon.md) |
| `meno sessions [--cwd DIR --status S --spawned-by ID\|me --title-prefix P --limit N]` | list sessions: id, status, name, directory. `show ID`, `cancel ID` and `answer ID QUESTION JSON` act on one session |
| `meno send (-s ID \| --new \| --fork SESSION) [options] TEXT` | write to a session and print the answer when the run ends; see below |
| `meno tail ID... [--children-of ID\|me --events T,... --from SEQ]` | print the events of sessions as JSON lines while they run |
| `meno doctor [--probe]` | read-only health check with suggested fixes |
| `meno update` | install the latest GitHub release |
| `meno -v`, `meno -h` | version, usage |

`sessions`, `send` and `tail` start the daemon when it is not running. They let you drive
meno from scripts and from other agents without the TUI.

The groups `mcp`, `tasks`, `ask`, `view` and `models` act on a session
(`MENO_SESSION`), which only an agent call has. See [the agent's tool](tools.md).

Run `meno <group> --help` for help; `--json` gives structured output. Exit codes: 0 success,
1 operation error, 2 invalid syntax, 3 forbidden, 5 a session waits for an answer, 6 the run was
stopped, 124 wait timed out (`meno doctor` also uses 4 for warnings).

## Driving sessions from scripts and agents

`meno send` needs a target: `-s ID` (an existing session), `--new` or `--fork SESSION`. There is no
implicit session. It runs until the run ends and prints the final answer; `--json` prints
`{sessionId, forkOf, status, text, question, usage, warnings}`. To work in the background wrap it in
a task: `meno tasks start "meno send --new --json ..."`. The task's notification, `tasks logs`,
`tasks wait` and `tasks cancel` are the background machinery.

- `--new` and `--fork` take `--cwd DIR`, `--title T`, `--system NAME` (a [system prompt](settings.md#system-prompts))
  and `--no-ask`. `--fork SESSION` copies the history of SESSION up to `--at N` (a turn number, default the last
  turn) into a new independent session, with its summary, model and settings. It writes nothing to the source.
  The fork is checked before it is created, so a bad request leaves no session behind.
- `--model PROVIDER/MODEL` and `--effort E` choose the model. With `-s` they change an idle session (the daemon
  refuses while it runs; the provider cache restarts). The provider must be configured, the model listed by
  `meno models`, and the effort one the model supports.
- `--timeout SECONDS` stops waiting and exits 124; the run goes on. `--detach` does not wait and, with `--new` or
  `--fork`, prints the new session id. If the session is already running, the message is queued as when you type
  while it works, and the command returns at once.
- If `send` is interrupted (Ctrl+C, SIGTERM, `meno tasks cancel`) it cancels the run of a session it created.
  `--keep-running` leaves it running; `--cancel-on-exit` also cancels the run of an existing session.
- When the agent of the session asks a question with `meno ask`, `send` prints what the agent said, exits 5, and the
  run stays open. Look at it with `meno sessions show ID` and reply with
  `meno sessions answer ID QUESTION '{"q1": "yes"}'` (or `--cancel`).

A session started by an agent's `meno send` records that agent's session as its origin (`spawnedBy`), shown by
`meno sessions --json` and used by `--spawned-by me`. The daemon limits such sessions: at most `max_spawn_depth`
(2) levels deep, and at most `max_workers` (8) per session that are running, waiting for an answer or not yet
written to. Over the limit `send` exits 3.

`meno sessions show ID` prints the state, model, the open question and the latest answer; `meno sessions cancel ID`
stops the run (an error when nothing runs). `meno tail` prints JSON lines `{session, boot, seq, type, data}` while a
session runs and ends with it; an idle session gives its turns. `--children-of ID|me` follows the sessions that ID
started until none runs. `--from SEQ --boot BOOT` continues after an event number of the same daemon run.

## meno doctor

`meno doctor [--probe] [--json]` checks the setup and prints what is wrong with a
suggested fix. It is read-only: it never logs in, restarts, repairs or edits
anything. It checks the daemon socket, version and protocol, the JSON config
files, the integrity of `meno.db`, providers and MCP servers. It works without a
running daemon. `--probe` also lists the models of each enabled provider (bounded
network calls). Secrets are never printed. Exit codes: 0 healthy, 4 warnings, 1 errors.

## meno acp

`meno acp` serves Meno as an Agent Client Protocol agent on stdin/stdout, see [ACP](acp.md). `--no-ask` creates sessions without `meno ask`.

## --no-ask

`meno --no-ask`, `meno send --new --no-ask` (and `--fork --no-ask`) and `meno acp --no-ask` create sessions in which `meno ask` is turned off: the tool answers with an error (exit 3) that tells the agent to write its question in the chat. The setting belongs to the session: it is stored, other windows see it, and a fork keeps it. It cannot be turned off later. A session you open with `/sessions` keeps the setting it was created with.
