# Settings (`:settings`)

| Setting | Values |
| --- | --- |
| js code | `all`, `lines <n>`, `off` — how much of the code the timeline shows |
| js output | `all`, `lines <n>`, `off` |
| js strings | `off` (default), `lines <n>` — string and template literals in tool-call source show their first `n` content lines and a muted `…` (`0` = marker only); display only; set it in `:settings` (`+`/`-` change `n`) |
| images | `auto`, `off`, `kitty`, `sixel`, `iterm` — see [terminal graphics](graphics.md) |
| thinking | `all`, `lines <n>` (the newest rows), `off` |
| js timeout | `<n> sec`, `off`, `model` — with `model` the tool gets a `timeout` argument and the model chooses per call |
| js timing | `all`, `slow only`, `off` — which finished js calls show their duration (slow: half of the limit or 10 s) |
| turn summary | `on`, `off` — the line that closes a run |
| notify sound | `default`, `bell`, or one of 25 built-in sounds; changing it previews the sound |
| notify | `off`, `question`, `done` — signal when the agent asks a question (`question`), and also when a turn ends, with success or error (`done`); not after your own Ctrl+C. Works for every session, also in the background |

See [notification sounds](notifications.md) for the 25 built-in cues, previews,
custom commands, and platform audio requirements.

`←`/`→` change the choice, `+`/`-` the number. `:theme` opens 20 built-in
color themes with a live preview.


Settings without a row in the dialog live in the settings JSON (`meno.db`):
`compact_percent` (80), `compact_keep` (8) and `compact_summary_tokens` (4000) tune the
automatic compaction safety net, see [sessions](sessions.md); `todo_collapsed` is
toggled with `:todo`.
