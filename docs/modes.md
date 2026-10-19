# Modes (`#plan`, `#build`, `#goal`)

Start a message with a mode and the request runs under it:

```
#plan add rate limiting to the API
#build
#goal make the whole test suite pass
```

The mode is the first word. Typing `#` at the start of the message completes it. The tag is
taken off the text (the transcript still shows it), and it applies to that request only; the
next message without a tag runs as usual and tells the model that the mode ended. A tag in
a message sent while the agent works is ignored, because a mode starts a request.

| Mode | What changes |
| --- | --- |
| `#plan` | **Read-only.** `fs.write`, `fs.patch` and `fs.edit` are refused, `tasks.start` and `agents.spawn` are off, and `sh` runs in a sandbox that cannot change files (outside the temporary directories). The model researches and answers with a plan. When the plan is done meno asks whether to start `#build`; typing an answer in that form sends it as another round of `#plan`. |
| `#build` | Carry the task out, run the project's checks and fix what fails. Written alone, `#build` means "carry out the plan above". |
| `#goal` | The message is a goal. The agent works until it calls `goal.done(summary)` or `goal.blocked(reason)`. An answer without either makes meno ask it to go on, at most 25 times; then the run stops and says so. The status bar shows `#goal 3/25`. |

The sandbox for `sh` is `sandbox-exec` on macOS and `bwrap` on Linux. Where neither exists,
`sh` is refused in a read-only mode instead of running unprotected. MCP tools are not limited:
a server you added can still change things.

## Your own modes

A mode is a markdown file in `$MENO_HOME/modes/` or `.meno/modes/` of the project (a project
file wins). The file name is the name of the mode; the text after the header is what the model
is told. The header is optional:

```
---
description: read the diff and report problems
readonly: true      # like #plan
goal: false         # keep going until goal.done / goal.blocked
rounds: 25          # the limit for a goal mode (at most 200)
effort: high        # reasoning effort for this request, if the model has the level
next: build         # offer this mode when the run ends
default: Review the latest commit.   # the task when the mode is written alone
---
Review the changes and list problems by severity. Do not fix anything.
```

A file named `plan.md`, `build.md` or `goal.md` replaces the builtin mode.
