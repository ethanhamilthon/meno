# ACP: Meno as an agent in other editors and clients

`meno acp` serves the Agent Client Protocol (ACP, version 1) on stdin and stdout.
An ACP client launches it as the agent command. Meno knows only the protocol: no
client gets special treatment. Diagnostics go to stderr; stdout carries only protocol
messages. The command starts the daemon of `MENO_HOME` when it is not running, and
never stops it.

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
reaches the model.

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

## Limits in 0.6.6

- MCP servers passed by the client in `session/new` are rejected with an error.
  Configure servers once with `meno mcp add`; they are available in ACP sessions.
- There is no per-command permission prompt: Bash runs with the same trust as in
  the terminal app, and the client does not get permission requests.
- The agent publishes no structured plan; there are no plan updates.
- A session is shared with other windows without an exclusive owner: a second
  window can send to it while an ACP prompt runs. A daemon restart interrupts running
  commands, as everywhere.
- Skill commands list the globally enabled skills, not skills of the session's project.
