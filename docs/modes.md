# Modes (`#brainstorm`, `#goal`)

A mode changes how the agent works on your messages. Type `#`, choose a mode
with Tab or Enter, and it appears in the input as one object. A typed `#name`
without the menu is plain text.

| Mode | What changes |
| --- | --- |
| `#brainstorm` | Explore options, compare benefits and risks, and write a complete spec in `.report/specs/`. Prototypes belong in a temporary directory. |
| `#goal` | Work until the agent calls `goal.done(summary)` or `goal.blocked(reason)`. An answer without either asks the agent to continue, at most 25 times. |
| `#none` | Clear the modes of the session. |

## Sets of modes

Several modes can be on at once, for example `#brainstorm #goal`. A message
that names modes replaces the whole set. A message without modes keeps the set.
`#none` clears it.

The set stays with the session after a restart. A new session starts without
modes. The status badge shows the set, for example `NORMAL #brainstorm #goal`.
Unknown saved mode names are ignored.

## Tool access

Both modes keep all tools. The rule to write a spec in `#brainstorm` is a
prompt instruction, not an access limit. The model can still change files.
MCP servers remain available in both modes.

Modes are built in. User mode files are not read.
