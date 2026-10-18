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
| `Ctrl+V`, `:attach [path]` | attach a PNG/JPEG from the clipboard or a file |
| `:new :fork :sessions :cd :compact :export` | sessions, see [sessions](sessions.md) |
| `:tasks :agents :todo` | background processes, working subagents, fold the plan |
| `:model :settings :theme :skills :mcp :providers` | dialogs |
| `:stop :q :q!` | stop the active request, quit, quit and cancel everything |

`:reset` no longer exists: every JS call starts fresh.

## Timeline

Answers are drawn as Markdown (headings, lists, quotes, alerts, tables, fenced code
with language colours). Every js call shows its duration, long calls turn yellow, and
a line sums up each run (time, requests, js calls, tokens, cache hit rate, cost).
The plan appears as a read-only block; steps and acceptance criteria are separate and
unverified criteria are called out. `:todo` folds it and the choice is kept.

## JS display

Tool-call source is highlighted by a JavaScript tokenizer once the whole call has been
received; while it streams, and when it cannot be tokenized (unterminated string or
template), it stays plain. Strings stay strings whatever they contain. The setting
`js strings` is `off` (default) or `lines <n>`: string and template literals show their
first `n` content lines followed by a muted `…` (`n = 0` shows only the marker); quotes,
interpolations and the surrounding code stay. Folding is display only: copy, export,
execution and model replay use the full source. Turn it on and off with the `js strings` setting.

## Images

`[Image #N]` tokens are one unit in the input. At most 5 images per message, 5 MB
each, PNG or JPEG, scaled to 1568 px. The agent can look with `view(path)`. With a
capable terminal the timeline shows the pictures themselves, see
[terminal graphics](graphics.md); otherwise a text block names number, size and type.
