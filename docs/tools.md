# The agent's tool

The model has one tool, `bash`. Each call runs one command in a fresh `/bin/sh` in the
session's working directory. Nothing is shared between calls: durable state lives in files
and in background tasks. The result ends with `[exit N]` or
`[cancelled]`. Output is capped once at the source (32 KB of head and 24 KB of
tail), and the full text is kept on disk. Already-sent tool results are not
shortened on later requests; automatic compaction still summarizes older
conversation when needed.

Calls have no time limit. All calls of one model response run in parallel and start after the
response is complete; results are stored in call order, and a failing call does not stop the
others. A message you write moves unfinished calls to background tasks (see [sessions](sessions.md)).

## The `meno` command

Inside a call, `meno` is the running executable. Run `meno <group> --help` for details and
add `--json` for structured output.

| Group | Commands |
| --- | --- |
| mcp | `servers`, `tools SERVER`, `call SERVER TOOL [ARGS]`, `results ID`, `add`, `remove`, `restart` |
| tasks | `start CMD`, `list`, `status ID`, `logs ID`, `wait ID`, `cancel ID` |
| ask | `meno ask FILE\|-` shows a form to the user and prints the answers |
| view | `meno view IMAGE` shows an image to the model |
| models | the models of the configured providers |

The other commands (`daemon`, `sessions`, `send`, `tail`) are for people and scripts, see
[the command line](cli.md). All normal tool groups are available without application modes.

Daemon-backed groups reach the daemon through the `MENO_SOCK`, `MENO_SESSION`, `MENO_TOKEN`
and `MENO_CALL` variables that the daemon puts in the environment of each call. The token is
valid for that session only. See [the daemon](daemon.md).

## Background tasks

`meno tasks start CMD` runs a command under the process supervisor. A task outlives the call
and the client. `meno tasks start` has no timeout (an old `--timeout` fails with an error). `meno tasks wait ID --timeout S` exits with 124 when its wait times out and does not cancel the task. See
[sessions](sessions.md) for what stop does to tasks.

## MCP

The daemon owns one MCP runtime shared by the interface and agent. Use `:mcp`
to create a server. Changes apply to the agent without restarting the daemon.
The dialog refreshes running, starting, and error status while open. Environment
variables and HTTP headers are edited as JSON objects with string values.

`meno mcp call` prints a JSON envelope with `text`, `structuredContent`, `isError`,
`resultId`, artifact paths, retained size, and any retention notes. Text and
structured content remain separate. A tool's own `isError` does not fail the
command. Transport and protocol errors do fail it. Images, audio, and binary
resources are stored as local files; use `meno view PATH` for supported images.
Use `meno mcp results ID` to page through the retained result. Results belong
to their session and are removed when that session is deleted.

`meno mcp add SERVER` enables an existing saved definition. `remove` deletes it;
`restart` reconnects it. Each asks for human approval. Unknown definitions must
be created with `:mcp`. File definitions remain read-only, but an enabled one
can be restarted. File edits require restarting the daemon.

## Files, the web and images

There are no file or web commands. The model reads and edits files with `cat`, `sed`, `rg` or
`grep` and quoted heredocs, and reaches the web with `curl` or an MCP server. `meno view` sends a
PNG or JPEG to the model (see [images](ui.md#images)).

## Skills and instructions

Skills are listed in the system prompt; the model reads one with `cat`. Your
The project `AGENTS.md` is added to the prompt as described in the [README](../README.md).
