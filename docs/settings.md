# Settings (`:settings`)

Settings live in tabs: **General**, **Display**, **Notify**, **Context**.
`Tab` or `]` moves to the next tab, `Shift+Tab` or `[` to the
previous one. `↑`/`↓` or `j`/`k` selects a row; `←`/`→`, space or Enter
changes a choice; `+`/`-` changes a number. Enter edits a text value (the
notify command), Enter saves it and Esc cancels. Every change is saved at once.

| Tab | Setting | Values |
| --- | --- | --- |
| General | theme | `meno`, `pure-black`, `catppuccin-mocha`, or a custom theme; live preview |
| General | images | `auto`, `off`, `kitty`, `sixel`, `iterm` — see [terminal graphics](graphics.md) |
| Display | bash code | `all`, `lines <n>`, `off` — how much command text the timeline shows |
| Display | bash output | `all`, `lines <n>`, `off` |
| Display | thinking | `all`, `lines <n>` (the newest rows), `off` |
| Notify | notify | `off`, `question`, `done` — see [notification sounds](notifications.md) |
| Notify | notify sound | `default`, `bell`, or one of 25 built-in sounds; changing it previews |
| Notify | notify volume | `20`…`100` percent; previews on change; ignored for a custom command |
| Notify | notify command | optional shell command that replaces the sound |
| Context | auto compact | the percentage of the context window at which meno compacts before a request |
| Context | daemon idle exit | minutes (default 30); 0 means never |

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
  `session.models()`; a stored effort the model does not take is replaced by
  `medium`, `high` or its first level. In `:login` the `run` command has its own field;
  `models` can only be set in `provider.json` (a saved provider keeps what it has).
- `mcp.json`: `[{"name", "transport", "command" or "url", "env", "headers", "enabled"}]`;
  `enabled` defaults to true, `transport` to `stdio`.
- `theme.json`: `[{"name", "dark", "bg", "fg", "dim", "blue", "purple", "green",
  "yellow", "red", "orange"}]` with colors as `RRGGBB`; the other shades are derived.

Settings without a row in the dialog live in the settings JSON (`meno.db`):
`compact_keep` (8) and `compact_summary_tokens` (4000) tune the automatic
compaction safety net, see [sessions](sessions.md).
