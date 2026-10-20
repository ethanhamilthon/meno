# The agent's tool

The model has one tool, `bash`. Each call runs one command in a fresh `/bin/sh` in the
session's working directory. Nothing is shared between calls: durable state lives in files
and in background tasks. The result ends with `[exit N]`, `[timed out after Ns]` or
`[cancelled]`. Output is bounded and the full text is kept on disk.

The `bash timeout` setting (default 120 seconds, `off` or `model`) limits a call; with
`model` the tool gets a `timeout` argument in seconds.

## The `meno` command

Inside a call, `meno` is the running executable. Run `meno <group> --help` for details and
add `--json` for structured output.

| Group | Commands |
| --- | --- |
| mcp | `servers`, `tools SERVER`, `call SERVER TOOL [ARGS]`, `results ID`, `add`, `remove`, `restart` |
| tasks | `start CMD`, `list`, `status ID`, `logs ID`, `wait ID`, `cancel ID` |
| ask | `meno ask FILE\|-` shows a form to the user and prints the answers |
| goal | `done SUMMARY`, `blocked REASON` (for the `#goal` mode) |
| view | `meno view IMAGE` shows an image to the model |
| models | the models of the configured providers |

The other commands (`daemon`, `sessions`, `send`, `tail`) are for people and scripts, see
[the command line](cli.md). The groups a session may use depend on its [modes](modes.md).

Daemon-backed groups reach the daemon through the `MENO_SOCK`, `MENO_SESSION`, `MENO_TOKEN`
and `MENO_CALL` variables that the daemon puts in the environment of each call. The token is
valid for that session only. See [the daemon](daemon.md).

## Background tasks

`meno tasks start CMD` runs a command under the process supervisor. A task outlives the call
and the client. `meno tasks wait ID` exits with 124 when its wait times out. See
[sessions](sessions.md) for what stop does to tasks.

## MCP

Configured MCP servers are listed in the system prompt with their tool signatures. A call
returns an envelope with `isError` and the content; a tool's own error does not fail the
command. Large results are stored and can be read again with `meno mcp results ID`.

## Files, the web and images

There are no file or web commands. The model reads and edits files with `cat`, `sed`, `rg` or
`grep` and quoted heredocs, and reaches the web with `curl` or an MCP server. `meno view` sends a
PNG or JPEG to the model (see [images](ui.md#images)).

## Skills and instructions

Skills are listed in the system prompt; the model reads one with `cat`. Your
`AGENTS.md` files are added to the prompt as described in the [README](../README.md).
