# ACP: Meno as an agent in other editors and clients

`meno acp` serves the Agent Client Protocol (ACP, version 1) on stdin and stdout.
An ACP client launches it as the agent command. Meno knows only the protocol: no
client gets special treatment. Diagnostics go to stderr; stdout carries only protocol
messages. The command starts the daemon of `MENO_HOME` when it is not running, and
never stops it.

`meno acp --no-ask` creates every new session with `meno ask` turned off: the agent asks
its questions in the chat, and `/answer` is never needed. Sessions opened with
`session/load` keep the setting they were created with.

ACP sessions are ordinary Meno sessions: they appear in `/sessions` and
`meno sessions`, use the same providers, skills, MCP servers and settings, and can
be opened in the terminal app.

## What works

| Feature | How |
| --- | --- |
| Chat, streaming | reply text arrives as message chunks |
| Tool calls | each Bash call is a tool card with its command, status, exit code and bounded output |
| Cancel | `session/cancel` stops the run of the current prompt; the answer is `cancelled` |
| Create, list, load | `session/new`, `session/list`, `session/load` (history is replayed, the working directory must match) |
| Models | `model` and `effort` config options; the model value names the provider and model, so equal ids of two providers never collide; changing them needs an idle session |
| Usage, cost, title | `usage_update` (context tokens, window, cost when the model is priced) and `session_info_update` |
| Thinking | exposed thinking summaries arrive as thought chunks; nothing hidden is invented |
| Slash commands | `/answer`, `/compact`, `/rename`, `/skills`, `/context`, `/help` and `/skill:<name> [text]` for every enabled skill |
| Images and context | image blocks (PNG, JPEG and other image types the model accepts), embedded text resources and resource links as references. Audio and binary resources are rejected with an error |
| Several sessions | each session has its own run; two connections can follow one session |

A prompt that begins with a known slash command is a control command and never
reaches the model. The command list is published with `available_commands_update`
right after the `session/new` or `session/load` response. A client that creates
the session only with the first message (Paseo's New workspace screen) shows the
commands from that point on, because ACP has no request for them before a session
exists.

## Questions (`meno ask`)

When the agent asks a question, the form is shown as a normal chat message with the
question id, the choices and a copyable reply. The current prompt ends there (the
stop reason is `end_turn`), but the run in the daemon stays paused. Reply with:

```
/answer <question-id> {"q1":"choice-id","q2":["a","b"],"q3":{"selected":["x"],"text":"more"}}
/answer <question-id> --cancel
```

A malformed or invalid answer keeps the question open. While a question is open,
other messages do not steer the agent; they point to the question. A client that
cannot show native forms is never treated as having rejected a question: nothing is
cancelled or answered for you.

If the connection closes while a question is open, the question and the paused run
stay in the daemon. Connect again, `session/load` the session and the question is
shown again. If the connection closes during a normal run, that run is stopped.

## Client-provided MCP

`session/new` and `session/load` accept stdio, streamable HTTP (`type: "http"`)
and SSE (`type: "sse"`) servers. These are **ephemeral, session-scoped** tools,
added alongside Meno's global MCP servers. They do not change `mcp.json`. A server
name that collides with a global server or another entry is rejected, not shadowed.
Names use Meno's existing format: 1–40 lowercase letters, digits, `-` or `_`.

Stdio requires an absolute executable path, `args` and `env` arrays; it runs in
the session's working directory. HTTP/SSE require a URL and a `headers` array.
Credentials are retained in memory only by the transports, not saved as session
configuration or chat history. Tool results still follow normal Meno persistence;
do not ask a tool to echo secrets. Configuration is limited to 16 servers and
256 KiB per entry; combined startup is bounded to 15 seconds. Invalid entries and
setup failures are errors; partial startup rolls back without replacing an existing
working overlay. Supply the configuration and credentials again on load.

Only one ACP attachment may own a session's client MCP at a time. A successful idle
load by that attachment replaces it; another connection cannot take it over. Empty
MCP lists remain supported, including loading a session paused on a question.
Disconnect, session deletion and daemon shutdown stop scoped transports and stdio
process groups. If the adapter crashes, a heartbeat lease expires within 45 seconds.
Global servers are not stopped. Replacing MCP while a run is active is rejected.

Client-supplied executables run as the local Meno user, with the ordinary inherited
process environment plus supplied variables. Connecting an ACP client therefore
requires the same trust as local Meno execution; there is no added permission or
sandbox system. The existing `--no-ask` behavior is unchanged.

## Protocol and diagnostics

Compatibility tests pin the official stable ACP v1 schema at upstream commit
`097a8a8e9f79998b56bff79ac3bb125b775e3c50` (2026-10-24). Meno supports version 1. If the
client requests another or no version, `initialize` answers with version 1 and the
client decides whether to continue; a non-numeric version is an invalid-params error.
Malformed optional client capabilities default to absent. Meno does not issue
client filesystem or terminal requests, regardless of those capabilities.

`session/list` filters by absolute working directory and returns up to 200 items
per page, ordered by descending update time then session ID. Pass `nextCursor` as
`cursor` with the same filter. Pagination is not snapshot-isolated across changes.
`session/load` replays the daemon's full available history; reconnect snapshots
recover missed assistant turns without repeating already delivered turns.

Set `MENO_ACP_DIAGNOSTICS=1` to emit JSON metadata to stderr: a known method name,
JSON-RPC outcome code and elapsed milliseconds. It is off by default. It never
logs raw requests, IDs, prompts, responses, thought text, commands, MCP configuration,
headers or credentials. Internal/provider errors use generic wire messages.
Normal protocol responses and tool cards still contain their intended content.

Deterministic stdio integration tests use local fake providers and sandbox MCP
servers, including a nonempty MCP payload. A live client/provider roundtrip
has not been validated for this release; client rendering and provider setup can
still fail independently. This is not a claim of complete ACP conformance.

## Limits

- There is no per-command permission prompt: Bash runs with the same trust as in
  the terminal app, and the client does not get permission requests.
- The agent publishes no structured plan; there are no plan updates. Fork/resume,
  audio, new modes and client filesystem/terminal integrations are not implemented
  or advertised. Existing model/effort selectors remain available.
- A session is shared with other windows without an exclusive owner: a second
  window can send to it while an ACP prompt runs. A daemon restart interrupts running
  commands, as everywhere.
- Skill commands list the globally enabled skills, not skills of the session's project.
