# Interface

INPUT is the default editor. A slash that starts a word (at the start of the
draft, or after a space or a newline) opens the command list above the composer,
anywhere in the text. Arrows pick a row, Tab completes it, Enter runs it and
removes the `/command` from the draft; the rest of the draft stays. Esc closes the
list and keeps the text. A command that fails keeps its text for correction.
URLs and paths (`http://x`, `/tmp/project`, `a/b`) are ordinary text.
Use `/skill:<name>` to insert a skill reference; `/skills` manages skills. A bare `/` lists every enabled skill after the commands. When a message with a skill reference is sent, the text of the skill goes into the conversation with it (the timeline shows `/skill:name loaded`), so the model does not have to read the file. This holds for a message that waits in the queue too. A bare name works too: `/pla` lists the enabled skills that start with `pla`, and Enter or Tab inserts the skill as one object without running or sending anything. A command with the same name wins. Skills marked `disable-model-invocation: true` are never offered to the model, only through this selection.
Old colon commands and bare `/name` skill references are ordinary text. Esc without a list keeps INPUT, the draft, and the cursor. INPUT is the only
mode: there is no NORMAL, EDIT, VISUAL or COMMAND mode.

The welcome screen shows the MENO wordmark, whose glyphs keep changing while its shape
stays, a tagline, the version (with the update notice when one exists), and one row of facts:
model and effort (or `/model`, or `/login` when no provider works), the folder, the number of
skills, the number of enabled MCP servers (with `N failed` in red when one cannot start) and
whether an `AGENTS.md` was found. The rest of the screen stays blank.

An empty composer shows one of three hints: `Type to start building anything` (new
session), `Tell the working agent something` (an agent is running) and
`Type your next task here` (the agent has answered). While an agent works, a green
wave travels across the dark-gray input background without changing text colors.
The composer has one blank padding row above and below the text. The footer
has no decorative rules or activity animation.

Text keeps two free columns on the left and on the right of every row: the
timeline, the composer, the status bar and the lists. Backgrounds, rules and icons
may use them. Ordinary words wrap as units; only overlong tokens split at
grapheme boundaries. Visual wrapping does not change stored text.

| Key or command | Action |
| --- | --- |
| Enter / Shift+Enter | Send / insert newline in INPUT |
| Enter during a run | Queue steering for the next model request |
| Ctrl+C | Stop this session's active work (quits on the first-run screen) |
| Ctrl+V, drag/drop, @ | Image attachment / file picker in INPUT |
| Drag, double click, triple click | Select text, a word or a row in the timeline or the composer; it is copied on release |
| ←/→ in /model | Preview an effort; Enter applies, Esc cancels |
| Esc in INPUT (no command list) | Keep typing without changing the draft |
| /new /sessions /fork /rename /cd /compact /export | Session actions |
| /model /settings /skills /mcp /login /tasks | Model and management dialogs |
| /context | ctx/max, spent, in/out and cache hit for the last turn, the session and the current request |
| /stop /q | Stop this turn; leave (work keeps running) |

A mouse selection is copied to the clipboard on release and stays highlighted
until the next key or click. The wheel, PgUp and PgDn scroll the timeline.

Dialogs opened by a command return to INPUT when they close. Errors retain the
command text for correction. Direct-shortcut dialogs preserve origin.

The footer shows the session title, or its folder when untitled. Automatic titles
replace file/folder/skill tags with readable labels before truncating.

## Status bar

Left: session title. Center: `N running | N tasks | N Asking | N done`.
Right: `model | effort | ctx/max` (effort is omitted when unavailable). Context
is retained before optional labels on narrow terminals. There are no mode or dialog badges.
Labels and counts use dim gray. Typed input stays bright.

- running: active sessions, excluding those waiting for an answer.
- tasks: live background Bash processes.
- Asking: sessions waiting for an agent question form.
- done: sessions with unread completed runs, including failures/stops, where the
  last accepted written message was yours from the TUI. ACP, agents, `meno send`,
  and automatic task wake-ups do not count. Queued messages follow acceptance
  order; messages left for a follow-up run count only for that run.

Counts cover the whole daemon, not only the current project. Zero counts are hidden.
Each counter keeps its own color on a narrow bar too, where it shrinks to `2r`; click one to open `/sessions` filtered to it. When the stream of the session breaks, the center reads `offline · state may be stale` in the warning color.
A session may have a new running request and an older unread completion. Blue
completion markers survive restart/reconnect and clear across UIs only after the
result is visible, not merely when a session is opened. Scrolling upward or opening
a modal does not silently mark unseen results read.

Usage metrics no longer appear in the status bar. `/context` shows current/latest
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

The setting `view` (settings, Display tab) picks how much of the work the timeline shows. It never
changes what is executed.

| view | shows |
| --- | --- |
| `full` | thinking, the title and the whole command of each call, and its output cut to five rows |
| `bash_only` | the title and the command of each call, cut to five rows; no thinking, no output |
| `compact` (default) | only the answers. While a turn runs, one row stands for the work: the start of the running command (or `thinking...`) on the left, the time of the whole turn on the right. The row goes when the turn ends |

The duration in the header of a call is always shown in `full` and `bash_only`.

## Rendering stability

The screen is redrawn by changing cells, never by clearing it, and the timeline only grows
at the end. A command block has no background of its own in any state: it appears with
the command, shows the output under it when it ends, and a failure turns the time in its
header red. A command that has ended stays on screen with its output until the saved turn
replaces it, so a block never leaves and returns. Streaming text and thinking only append.
Two things can still repaint: pictures on terminals that draw them with Sixel or iTerm
graphics, whose pixels are removed by repainting the cells under them (see
[terminal graphics](graphics.md)), and a terminal that does not support synchronized updates.

## Images

`[Image #N]` tokens are one unit in the input. At most 5 images per message, 5 MB
each, PNG or JPEG, scaled to 1568 px. The agent can look with `meno view PATH`. With a
capable terminal the timeline shows the pictures themselves, see
[terminal graphics](graphics.md); otherwise a text block names number, size and type.

## Exporting a session

`/export [path]` writes Markdown asynchronously. The default is `meno-session.md`
in the session's working directory; relative paths use that same directory.
Switching sessions while export runs does not change its source or destination.

The export contains the saved conversation, including archived turns retained
through compaction, tool source/results, available system events, and a table of
model requests. Each logical step has separate numbered attempts for retries;
summary-generation requests are labeled `compaction`. Historical provider, model,
effort, and effective system prompt/function definitions are retained per request,
with shared configurations printed once. Current session settings are labeled
separately and are never substituted for missing historical details.

Input tokens already include cache reads and writes: do not add cache counts to
input again. `unavailable` means the provider did not report the data (or older
session data did not retain it), not zero. A reported zero remains zero. Exports
made during active work stop at the last persisted event, mark incomplete work,
and do not include uncommitted streaming output. Interrupted attempts may lack a
final outcome or usage. Older sessions can have unavailable request details or
partial conversation history; the export warns rather than inventing data.

**Privacy:** system prompts can contain private project instructions and local
paths. Review exports before sharing. Export does not include provider API keys,
authorization headers, raw provider configuration, opaque replay state, or image
bytes; text intentionally present in prompts and the conversation is not redacted.
