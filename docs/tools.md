# The agent's tool

The model has one tool, `bash`. Each call runs one command in a fresh `/bin/sh` in the
session's working directory. Nothing is shared between calls: durable state lives in files
and in background tasks. The result ends with `[exit N]` or
`[cancelled]`. Output is capped once at the source (32 KB of head and 24 KB of
tail); when it was cut, the full text is kept on disk and its path is in the result.
Tool results are not shortened on later requests until the context nears the window: see
[elision of old output](#elision-of-old-output). Automatic compaction still summarizes older
conversation when needed.

The working rules (editing, verification, output) are part of the system prompt, not of the tool description. `meno doctor` is available next to `meno mcp`, `tasks`, `ask`, `view`, `models`, `sessions`, `send` and `tail`.

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
| ask | `meno ask FILE\|-` shows a form to the user and prints the answers; a session started with `--no-ask` refuses it (exit 3) and the agent asks in the chat |
| view | `meno view IMAGE` shows an image to the model |
| models | the models of the configured providers |

An agent starts another agent with `meno -p "PROMPT"` (it needs your explicit permission to do so), see
[the command line](cli.md#meno--p). All normal tool groups are available without application modes.

The groups reach the engine that runs the call through a descriptor the call inherits (`MENO_FD`),
with `MENO_SESSION`, `MENO_TOKEN` and `MENO_CALL` in the environment of each call. The token is
valid for that session only.

## Elision of old output

A long session keeps every tool result in the context. When the estimated request reaches
`elide_percent` of the model's window (default 50%), meno first cuts the output of old
calls: outputs over 2 KB keep their first and last 512 bytes and say how many bytes were left
out. The newest 6 turns, small outputs and the output of failed calls are never cut. Only what
the model receives is shorter: the timeline, the export and the compaction transcript still
hold the whole output. An output under 56 KB is not kept on disk, so the cut text says to run
the command again if it is needed.

The cut moves in steps, only when it saves at least 8 KB, because changing old messages costs
one provider-cache miss; between steps the messages sent are identical, so the cache keeps
working. If this is not enough, compaction (a summary) follows at its own threshold. The cut
is kept in memory: after meno restarts the first long request makes it again.

## Background tasks

`meno tasks start CMD` runs a command under the process supervisor. A task outlives the call
and the client. `meno tasks start` has no timeout (an old `--timeout` fails with an error). `meno tasks wait ID --timeout S` exits with 124 when its wait times out and does not cancel the task. See
[sessions](sessions.md) for what stop does to tasks.

## MCP

A server that declares capabilities without `tools` (only resources or prompts) starts
without a tool list; those features are not used yet. An HTTP session the server has
forgotten (404) is restarted once with a new `initialize`. Tools keep `title`,
`outputSchema` and `annotations` as the server sent them; annotations are hints and
never grant permission.

Each meno process owns one MCP runtime shared by the interface and agent. Use `/mcp`
to create a server: it asks name, transport, then command and arguments (stdio) or URL, one at a time,
and `cwd`, `env` and `headers` are edited afterwards in the server form. Changes apply to the agent without restarting meno.
The dialog refreshes running, starting, and error status while open. Environment
variables and HTTP headers are edited as JSON objects with string values.

The agent cannot create a server with `meno mcp add`: that command only enables a saved
server, and the user is asked first. To create one, the agent edits `$MENO_HOME/mcp.json`
(the format is in [settings](settings.md)) and runs `meno mcp restart NAME`, which does not
ask. The file is read on every call, so no restart is needed. An entry that is not
valid is skipped with a warning that `meno doctor` shows; a file that is not valid JSON lists
no servers until it is fixed. `meno mcp tools SERVER TOOL` prints the full description and
input schema of one tool; `meno mcp tools SERVER` lists one line per tool.

`meno mcp call` prints a JSON envelope with `text`, `structuredContent`, `isError`,
`resultId`, artifact paths, retained size, and any retention notes. Text and
structured content remain separate. A tool's own `isError` does not fail the
command. Transport and protocol errors do fail it. Images, audio, and binary
resources are stored as local files; use `meno view PATH` for supported images.
Use `meno mcp results ID` to page through the retained result. Results belong
to their session and are removed when that session is deleted.

`meno mcp add SERVER` enables an existing saved definition and `remove` deletes
it; both ask for human approval. `restart` reconnects a server that is already
configured and runs without a question, so an agent can recover a server that
stopped answering. Unknown definitions must be created with `/mcp`. A server
you added to `mcp.json` by hand is an ordinary definition; meno sees the edit at the next read.

A stdio server that answers slowly is not a failure: the handshake and the tool
list wait up to 30 seconds, and a server that writes to a pipe gets an unbuffered
stdout, so a Python server answers at once. When a server cannot start, the
error carries the last lines it wrote to stderr.

## Files, the web and images

There are no file or web commands. The model reads and edits files with `cat`, `sed`, `rg` or
`grep` and quoted heredocs, and reaches the web with `curl` or an MCP server. `meno view` sends a
PNG or JPEG to the model (see [images](ui.md#images)).

## Skills and instructions

Skills are found in `.meno/skills` and `~/.meno/skills`, and also in the shared
`.agents/skills` of the project and of your home directory (not when `MENO_HOME` is
custom). Order, lowest to highest: `~/.agents/skills`, `~/.meno/skills`,
`<project>/.agents/skills`, `<project>/.meno/skills`; a later skill replaces an earlier one
with the same name. `SKILL.md` frontmatter is read as YAML. A skill with problems still
loads; `meno doctor` lists the warnings. After compaction the summary lists the skill
files the model had read, so it can read them again.

Skills are listed in the system prompt; the model reads one with `cat`. Your
The project `AGENTS.md` is added to the prompt as described in the [README](../README.md).
