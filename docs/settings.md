# Settings (`/settings`)

Settings live in tabs: **General**, **Display**, **Notify**, **Context**.
`Tab` or `]` moves to the next tab, `Shift+Tab` or `[` to the
previous one. `↑`/`↓` or `j`/`k` selects a row; `←`/`→`, space or Enter
changes a choice; `+`/`-` changes a number. Enter edits a text value (the
notify command), Enter saves it and Esc cancels. Every change is saved at once.

Look and signal settings (`theme`, `notify`, `notify sound`, `notify volume`, `notify command`) can be kept for one window: press `s` to switch the scope between *shared* and *this window*, and change the setting. A window override is never sent to the daemon and survives a refresh of the shared settings; an edit in the shared scope replaces it. `r` drops every override of the window.

| Tab | Setting | Values |
| --- | --- | --- |
| General | theme | `default` or a custom theme from `theme.json`; live preview |
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

Providers, MCP servers and themes live in JSON files in `$MENO_HOME`, next to `meno.db`.
`/login`, `/mcp` and `meno mcp` read and write `provider.json` and `mcp.json`; you can also edit
them by hand, and the next request or dialog read sees the change (no restart). `theme.json` is
read only. A bad entry gives a notice and is skipped, and it stays in the file when meno saves
the others. A file that is not valid JSON is reported and never overwritten: fix it and meno
saves again. A key written as `env:NAME` is read from the environment and stays that text in the
file. API keys are otherwise stored as plain text, so keep `$MENO_HOME` private and do not commit
these files with keys in them. Files are written with mode 0600.

- `provider.json`: `[{"name", "base_url", "api_key", "protocol", "run", "models"}]`. `api_key` may be
  `env:NAME` to read an environment variable. `run` is a shell command meno starts at launch
  (for a local proxy) and stops when it exits; the output goes to `$MENO_HOME/providers/<name>.log`.
  meno does not wait for it: a request that fails while the proxy starts is retried (a provider with
  `run` gets 9 attempts, about a minute and a half). `models` holds your own settings by model id:
  `{"gpt-x": {"context": 128000, "input": 1.25, "output": 10, "cache_read": 0.125, "cache_write": 1.5,
  "vision": true, "efforts": ["low", "high"]}}`. Prices are USD per million tokens; a field you leave out
  keeps the value of the model catalog, and a model without a catalog entry is priced only when both
  `input` and `output` are set. `efforts` replaces the standard levels for that model in `/model`,
  `session.models()`; a stored effort the model does not take is replaced by
  `medium`, `high` or its first level. In `/login` the `run` command has its own field;
  `models` can only be set in `provider.json` or by editing the file.
- `mcp.json`: `[{"name", "transport", "command" or "url", "args", "cwd", "env", "headers", "enabled"}]`;
  `enabled` defaults to true, `transport` to `stdio`. A stdio server may name its
  arguments and its working directory separately (`"command": "python3"`,
  `"args": ["server.py"]`), the usual shape of a server configuration. Without
  `args` the whole `command` is used as it is.
- `theme.json`: `[{"name", "dark", "bg", "fg", "dim", "blue", "purple", "green",
  "yellow", "red", "orange"}]` with colors as `RRGGBB`; the other shades are derived.

Settings without a row in the dialog live in the settings JSON (`meno.db`):
`compact_keep` (8) and `compact_summary_tokens` (4000) tune the automatic
compaction safety net, see [sessions](sessions.md).

## Web

- `web_search`: `PROVIDER/MODEL` used by `meno web search`. Empty means off.
- `web_allow_local`: let `meno web fetch` reach loopback and private addresses
  (the cloud metadata address stays blocked).
