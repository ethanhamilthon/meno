# Interface

Insert mode types text; `Esc` leaves it for Normal mode, which edits the input like
vim (operators with motions and text objects, `p u ctrl+r`, counts, `f t ; ,`, `v V
ctrl+v`). `k` on the first input row continues into the read-only timeline, which
supports every motion and yank (`y` copies through OSC 52). `:` opens commands.

| | |
| --- | --- |
| `Enter` / `Shift+Enter` | send / new line |
| `Enter` while the agent works | steering: queued above the input, delivered before the next model request; `↑` on an empty input takes it back |
| `Ctrl+C` | abort this session's active request |
| `Ctrl+V`, drag and drop, `@` | attach a PNG/JPEG from the clipboard, a dropped file or the picker |
| `:new :fork :rename :sessions :cd :compact :export` | sessions, see [sessions](sessions.md) |
| `:tasks` | background processes |
| `:model :settings :skills :mcp :login` | dialogs |
| `:snippets :save_snippet <name> :paste_snippet <name>` | saved text: list, save the selection, insert at the cursor |
| `:stop :q :q!` | stop the active request, quit, quit and cancel everything |


The composer shares the terminal background. Its top row is a `─` rule with the session title (saved title or first user message) at the right; long names are limited to 30% of the terminal width. Before a session has a title, the rule shows the working-directory name, or `~` for the home directory.

**Keyboard layouts.** Commands are matched by the key's position, so on a Russian or
Ukrainian layout `о` is `j`, `ж` (Shift) is `:`, and the command name after `:` is
typed in either layout (`:ьщвуд` is `:model`). Text you type (the input, search
boxes, form fields, command arguments, the target of `f t r`) is never translated.
`Ctrl`+letter of another layout works where the terminal reports the letter (kitty
keyboard protocol); other terminals send a control code or nothing for it.

**Mouse.** The wheel scrolls the timeline. Drag over the timeline or the composer
to select text (it is a visual selection and is copied through OSC 52 when you let
go), double-click selects a word, triple-click a line; `Esc` or the next click
clears it. The status bar and dialogs are not selectable with the mouse.

**Snippets.** Select text with `v`, `V`, `Ctrl+V` or the mouse, then
`:save_snippet <name>` stores it. `:paste_snippet <name>` inserts it into the
composer at the cursor and returns to insert mode. `:snippets` lists them:
Enter pastes, `r` renames, `d` twice deletes. Snippets are global and live in
`meno.db`; a name uses letters, digits, dot, dash or underscore.

## Status bar

Left: a colored badge with the mode (`INSERT`, `NORMAL`, …) followed by the modes of the
session, for example `NORMAL #brainstorm #goal`. An open dialog shows its own name instead
(`SESSIONS`, `HELP`, …). On a narrow terminal the modes leave the badge first.

Next to it are groups of the form `N x ◆` written without spaces (`3x◆`):

- green, blinking: `1x◆` — the agent of this session is working;
- purple, blinking: `Nx◆` — N background tasks of this session;
- after a `|`: `Kx◆` (green, blinking) is the number of other sessions that are
  running, and `D◉` (blue) the number of other sessions that finished since you last
  looked at them. A session that waits for an answer is not counted here; see `:sessions`.

Center: tokens in/out, cache share, context used/window and the spend. Right: the
model, with its effort in brackets.

## Timeline

Answers are drawn as Markdown (headings, lists, quotes, alerts, tables, fenced code
with language colours). Every bash call shows a small badge in the top right corner of its block: `⠹ 2.3s/120s` while it
runs (time so far / limit), then `✓ 2.3s` or `✕ 2.3s`; long calls turn yellow. A line sums up
each run (time, requests, tool calls, tokens, cache hit rate, cost).

## Tool-call display

The timeline shows the command of each bash call once the whole call has arrived; while it
streams, it stays plain. The settings `bash code`, `bash output`, `bash strings` and
`bash timing` (see [settings](settings.md)) choose how much of the command, its output and its
duration you see. Display settings never change what is executed or what the model sees.

## Images

`[Image #N]` tokens are one unit in the input. At most 5 images per message, 5 MB
each, PNG or JPEG, scaled to 1568 px. The agent can look with `meno view PATH`. With a
capable terminal the timeline shows the pictures themselves, see
[terminal graphics](graphics.md); otherwise a text block names number, size and type.
