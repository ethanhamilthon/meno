# Interface

INPUT is the default editor. A colon that starts a word (at the start of the
draft, or after a space or a newline) opens the command list above the composer,
anywhere in the text. Arrows pick a row, Tab completes it, Enter runs it and
removes the `:command` from the draft; the rest of the draft stays. Esc closes the
list and keeps the text. A command that fails keeps its text for correction.
A colon inside a word (`http://x`, `a:b`) is ordinary text. Esc without a list keeps INPUT, the draft, and the cursor. Only `:edit`
enters EDIT. There is no NORMAL or COMMAND mode.

An empty composer shows one of three hints: `Type to start building anything` (new
session), `Tell the working agent something` (an agent is running) and
`Type your next task here` (the agent has answered). While an agent works, a green
wave runs in phase across the composer header and footer, including their text.
The compact composer has no blank row between its input and footer.

Text keeps two free columns on the left and on the right of every row: the
timeline, the composer, the status bar and the lists. Backgrounds, rules and icons
may use them. Ordinary words wrap as units; only overlong tokens split at
grapheme boundaries. Visual wrapping does not change stored text.

| Key or command | Action |
| --- | --- |
| Enter / Shift+Enter | Send / insert newline in INPUT |
| Enter during a run | Queue steering for the next model request |
| Ctrl+C | Stop this session's active work |
| Ctrl+V, drag/drop, @ | Image attachment / file picker in INPUT |
| :edit | Vim-style navigation and editing of the draft (history when the draft is empty); no selection until `v` |
| v / V / Ctrl+V in EDIT | Start a selection; Esc returns to EDIT |
| Esc in EDIT | Return to INPUT |
| Esc in INPUT (no command list) | Keep typing without changing the draft |
| :new :sessions :fork :rename :cd :compact :export | Session actions |
| :model :settings :skills :mcp :login :tasks | Model and management dialogs |
| :context | ctx/max, spent, in/out and cache hit for the last turn, the session and the current request |
| :stop :q :q! | Stop, quit, or quit and cancel work |

VISUAL retains motions, counts, text objects and copy. History is read-only.
Completed copy/edit returns to INPUT. Mouse selection copies on release; the wheel
scrolls history. The command editor does not select transcript text on drag.

Dialogs opened by a command return to INPUT when they close. Errors retain the
command text for correction. Direct-shortcut dialogs preserve origin.

The composer header uses a dark gray line and session label. Automatic titles
replace file/folder/skill tags with readable labels before truncating.

## Status bar

Left: `model | effort` (effort is omitted when unavailable). Center:
`N running | N tasks | N Asking | N done`. Right: `ctx/max`, retained before
optional labels on narrow terminals. There are no mode or dialog badges.
Labels and counts use dim gray; rules use a darker gray. Typed input stays bright.

- running: active sessions, excluding those waiting for an answer.
- tasks: live background Bash processes.
- Asking: sessions waiting for an agent question form.
- done: sessions with unread completed runs, including failures/stops.

Counts cover the whole daemon, not only the current project. Zero counts are hidden.
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
running, terminal background on success, dark red on error/timeout/stop.
Output remains on its ordinary background. Stderr alone does not imply failure.

Bash source stays hidden while arguments arrive. The complete command and title
appear when execution starts, with Bash-aware highlighting. Very large source
may remain plain. Assistant text and thinking still stream. This does not change Markdown or highlight arbitrary
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
