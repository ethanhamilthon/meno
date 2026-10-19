# Modes (`#subagents`, `#brainstorm`, `#goal`)

A mode changes how the agent works on your messages. Pick it from the menu: type `#`,
choose a mode with Tab or Enter, and it appears in the input as one object, like a skill.
A typed `#name` without the menu is plain text.

| Mode | What changes |
| --- | --- |
| `#subagents` | The agent orchestrates. It has no file, shell, web or MCP access: it starts subagents with `agents.spawn` (in parallel where it can), waits for their results, starts the next work, and ends with a reviewer. A subagent does the work with the normal tools. |
| `#brainstorm` | The agent explores before anything is built: options with pros, cons and risks, prototypes only in a temporary directory, and at the end one complete spec in `.report/specs/`. The spec says what to build and how the parts fit; another agent writes the code. Only the prompt changes: the tools stay. |
| `#goal` | The message is a goal. The agent works until it calls `goal.done(summary)` or `goal.blocked(reason)`. An answer without either makes meno ask it to go on, at most 25 times. It does not nudge while subagents are still working: their events wake it. |
| `#none` | Clears the modes of the session. |

## Sets of modes

Several modes can be on at once: pick them in one message, for example `#subagents #goal`.
A message that names modes **replaces the whole set**. If `#subagents #brainstorm` is on and
you send `#goal #subagents ship`, the set is now `goal` and `subagents`; `brainstorm` is gone.
A message without modes keeps the set. `#none` empties it.

The set belongs to the session: it stays until it is changed, including after a restart. A new
session starts with no modes. The badge at the left of the status bar shows the set, for example `NORMAL #subagents #goal`.

## Limits

- `subagent_max_spawn` in the settings limits how many subagents one request may start (default 50).
  A refused spawn returns an error to the agent, which can then work with the results it has.
  The count starts again with each new request; a wake-up does not reset it. The setting has no row in `:settings`.
- The tool list follows the mode: a mode that removes a global makes it unavailable, and the
  description of the js tool says which globals are gone.
- `#brainstorm` does not restrict the tools. Its rule to write only the spec is in the prompt,
  not enforced; the model can still change files.
- MCP servers are not limited by a mode, except that `#subagents` has no `mcp` global.

Modes are built in: there are no user files for them.
