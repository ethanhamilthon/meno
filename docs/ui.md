# Interface

INPUT is the default editor. Esc toggles INPUT and COMMAND. Both use the same
composer with separate drafts and carets. A colon typed in INPUT is ordinary text;
COMMAND has a permanent `:` prefix. Completion appears above the composer.
There is no NORMAL mode.

| Key or command | Action |
| --- | --- |
| Enter / Shift+Enter | Send / insert newline in INPUT |
| Enter during a run | Queue steering for the next model request |
| Ctrl+C | Stop this session's active work |
| Ctrl+V, drag/drop, @ | Image attachment / file picker in INPUT |
| :v / :visual | Select the draft; with an empty draft, select transcript history |
| Esc in VISUAL | Return to INPUT |
| :new :sessions :fork :rename :cd :compact :export | Session actions |
| :model :settings :skills :mcp :login :tasks | Model and management dialogs |
| :context | Context occupancy, input/output, cache read/write and spending |
| :stop :q :q! | Stop, quit, or quit and cancel work |

VISUAL retains motions, counts, text objects and copy. History is read-only.
Completed copy/edit returns to INPUT. Mouse selection copies on release; the wheel
scrolls history. The command editor does not select transcript text on drag.

Successful new/session/fork/model selection returns to INPUT. Management dialogs
and commands return to COMMAND; canceling a COMMAND-opened dialog returns there.
Errors retain the command for correction. Direct-shortcut dialogs preserve origin.

The composer header uses a dark gray line and session label. Automatic titles
replace file/folder/skill tags with readable labels before truncating.

## Status bar

Left: INPUT, COMMAND or VISUAL (or an open dialog's name). Center:
`N running | N tasks | N Asking | N done`. Right: model and effort where space allows.

- running is green: active sessions, excluding those waiting for an answer.
- tasks is purple: live background Bash processes.
- Asking is orange: sessions waiting for an agent question form.
- done is blue: sessions with unread completed runs, including failures/stops.

Counts cover the whole daemon, not only the current project. Zeros use dark gray.
A session may have a new running request and an older unread completion. Blue
completion markers survive restart/reconnect and clear across UIs only after the
result is visible, not merely when a session is opened. Scrolling upward or opening
a modal does not silently mark unseen results read.

Usage metrics no longer appear in the status bar. `:context` shows current/latest
run usage, cumulative session usage and latest context estimate/window. Unknown
usage/pricing stays unknown; partial costs are identified as partial.

## Timeline and Bash

Answers render as Markdown. Each Bash call has a full-width dark gray header:
short action title on the left, live/final duration on the right. This timing is
always visible. Only the source region changes background: dark green while
streaming/running, terminal background on success, dark red on error/timeout/stop.
Output remains on its ordinary background. Stderr alone does not imply failure.

Finalized Bash source uses Bash-aware highlighting. Streaming/incomplete or very
large source may remain plain. This does not change Markdown or highlight arbitrary
stdout/stderr as shell code. Copying code excludes title, rules and timing.

Every run ends with input/output, cache read/write and cost on the left, and total
wall time plus Bash invocation count on the right. Narrow terminals wrap rather
than discard metrics. A shell script counts as one Bash invocation. Unknown cost
is `$?`; a priced subtotal with unpriced usage has `+?`. These are UI metadata, not
model-authored text or additional conversation context.

The settings `bash code`, `bash output` and `thinking` control retained source,
output and thought display. Display settings never change what is executed.

## Images

`[Image #N]` tokens are one unit in the input. At most 5 images per message, 5 MB
each, PNG or JPEG, scaled to 1568 px. The agent can look with `meno view PATH`. With a
capable terminal the timeline shows the pictures themselves, see
[terminal graphics](graphics.md); otherwise a text block names number, size and type.
