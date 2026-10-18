# Settings (`:settings`)

| Setting | Values |
| --- | --- |
| js code | `all`, `lines <n>`, `off` — how much of the code the timeline shows |
| js output | `all`, `lines <n>`, `off` |
| js strings | `off` (default), `lines <n>` — string and template literals in tool-call source show their first `n` content lines and a muted `…` (`0` = marker only); display only; set it in `:settings` (`+`/`-` change `n`) |
| images | `auto`, `off`, `kitty`, `sixel`, `iterm` — see [terminal graphics](graphics.md) |
| thinking | `all`, `lines <n>` (the newest rows), `off` |
| js timeout | `<n> sec`, `off`, `model` — with `model` the tool gets a `timeout` argument and the model chooses per call |
| js timing | `all`, `slow only`, `off` — which finished js calls show their duration in the corner of the block (slow: half of the limit or 10 s) |
| turn summary | `on`, `off` — the line that closes a run |
| notify sound | `default`, `bell`, or one of 25 built-in sounds; changing it previews the sound |
| notify | `off`, `question`, `done` — signal when the agent asks a question (`question`), and also when a turn ends, with success or error (`done`); not after your own Ctrl+C. Works for every session, also in the background |

See [notification sounds](notifications.md) for the 25 built-in cues, previews,
custom commands, and platform audio requirements.

`←`/`→` change the choice, `+`/`-` the number. `:theme` opens 20 built-in
color themes with a live preview.

## Files that add to the app

Three optional files in `$MENO_HOME` add providers, MCP servers and themes without
touching `meno.db`. They are read at start, only add (a name the app already has is
ignored with a notice), are read only in the dialogs (marked `from provider.json` /
`mcp.json`) and are never saved to the database. A broken file or entry gives a
notice and the rest still loads.

- `provider.json`: `[{"name", "base_url", "api_key", "protocol", "run", "models"}]`. `api_key` may be
  `env:NAME` to read an environment variable. `run` is a shell command meno starts at launch
  (for a local proxy) and stops when it exits; the output goes to `$MENO_HOME/providers/<name>.log`.
  meno does not wait for it: a request that fails while the proxy starts is retried (a provider with
  `run` gets 9 attempts, about a minute and a half). `models` holds your own settings by model id:
  `{"gpt-x": {"context": 128000, "input": 1.25, "output": 10, "cache_read": 0.125, "cache_write": 1.5,
  "vision": true, "efforts": ["low", "high"]}}`. Prices are USD per million tokens; a field you leave out
  keeps the value of the model catalog, and a model without a catalog entry is priced only when both
  `input` and `output` are set. `efforts` replaces the standard levels for that model in `:model`,
  `session.models()` and `agents.spawn`; a stored effort the model does not take is replaced by
  `medium`, `high` or its first level. In `:providers` the `run` command has its own field;
  `models` can only be set in `provider.json` (a saved provider keeps what it has).
- `mcp.json`: `[{"name", "transport", "command" or "url", "env", "headers", "enabled"}]`;
  `enabled` defaults to true, `transport` to `stdio`.
- `theme.json`: `[{"name", "dark", "bg", "fg", "dim", "blue", "purple", "green",
  "yellow", "red", "orange"}]` with colors as `RRGGBB`; the other shades are derived.


Settings without a row in the dialog live in the settings JSON (`meno.db`):
`compact_keep` (8) and `compact_summary_tokens` (4000) tune the
automatic compaction safety net, see [sessions](sessions.md).
