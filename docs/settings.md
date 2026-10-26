# Settings (`/settings`)

Settings appear in one scrollable panel above the composer. `↑`/`↓` or `j`/`k`
selects a row; `←`/`→`, space or Enter changes a choice; `+`/`-` changes a number.
Enter edits a text value, Enter saves it and Esc cancels. Every change is saved
at once. Esc closes the panel without losing your composer draft.

Notification settings can be kept for one window: `s` switches between *shared*
and *this window*. Window overrides are not sent to the daemon and survive
refreshes; a shared edit replaces the override. `r` drops all window overrides.

| Setting | Values |
| --- | --- |
| system prompt | `default` (built-in) or a file of `system-prompts/`; see below |
| view | `full`, `bash_only`, `compact` (default); see [the interface](ui.md#timeline-and-bash) |
| notify | `off`, `question`, `done`; see [notifications](notifications.md) |
| notify sound | `default`, `bell`, or one of 25 built-in sounds; previews on change |
| notify volume | `20`…`100` percent; ignored for a custom command |
| notify command | optional shell command replacing the sound |
| auto compact | context percentage at which meno compacts before a request |

The daemon exits after 10 continuous seconds without an attached event-stream
client, active session work, or running background task. An in-flight request,
reconnection, or new work restarts the grace period. Saved sessions and finished
tasks do not keep it alive. The next launch restores saved sessions. There is no
idle-timeout setting; old `daemon_idle_exit_min` values, including zero, are ignored.

## Palette

There is one palette in `$MENO_HOME/theme.json` (normally `~/.meno/theme.json`),
not a list of selectable themes. It starts with these colors:

```json
{
  "dark": true,
  "bg": "17191C",
  "fg": "DBDFE5",
  "dim": "888F99",
  "blue": "8CB8FF",
  "purple": "B7A5DE",
  "green": "9DCC9D",
  "yellow": "E1CD8C",
  "red": "E39A9A",
  "orange": "E9BC86"
}
```

Edit the file to recolor every open window using that home, normally within two
seconds; no restart or selection is needed. Colors accept `RRGGBB` or `#RRGGBB`.
Surface shades are derived. Invalid or incomplete saves keep the last valid
palette and show a notice; saving a valid object recovers automatically. The file
is initialized if missing at startup. An invalid existing file is never replaced.

On upgrade, an old array is backed up unchanged to `theme.json.legacy.bak` and
converted to one object: its valid `panel-dark` entry, or the new default palette.
The backup retains other custom themes. Restore it before downgrading to a
version that expects an array. Named-theme settings/window overrides are retired.

## Files that add to the app

Providers, MCP servers and themes live in JSON files in `$MENO_HOME`, next to `meno.db`.
`/login`, `/mcp` and `meno mcp` read and write `provider.json` and `mcp.json`; you can also edit
them by hand, and the next request or dialog read sees the change (no restart). `theme.json` reloads automatically. A bad entry gives a notice and is skipped, and it stays in the file when meno saves
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
- `mcp.json`: `[{"name", "transport", "command" or "url", "args", "cwd", "env", "headers", "protocol", "enabled"}]`;
  `enabled` defaults to true, `transport` to `stdio`. A stdio server may name its
  arguments and its working directory separately (`"command": "python3"`,
  `"args": ["server.py"]`), the usual shape of a server configuration. Without
  `args` the whole `command` is used as it is. `protocol` is `auto` (default: try the
  2026-07-28 protocol first, fall back to the legacy `initialize` handshake), `legacy`
  (handshake only, versions 2024-11-05 to 2025-06-18; use it for a server that
  misbehaves when probed) or `modern`. A silent legacy stdio server is started twice and
  can delay startup by up to 10 seconds; set `legacy` to avoid that. The modern protocol
  supports tools only; a tool that asks for more input is reported as an error.
- `theme.json`: one palette object, described above.

Settings without a row in the dialog live in the settings JSON (`meno.db`):
`compact_keep` (8) and `compact_summary_tokens` (4000) tune the automatic
compaction safety net, see [sessions](sessions.md). `max_spawn_depth` (2) and
`max_workers` (8) limit sessions that agents start with `meno send`, see
[the command line](cli.md#driving-sessions-from-scripts-and-agents).

## System prompts

The system prompt starts with one identity line: *You are Meno, a coding agent.* A custom
system prompt replaces only that line. What follows it stays: the skills, the project
`AGENTS.md`, the working rules, the MCP tools and the environment.

A custom prompt is a Markdown file `$MENO_HOME/system-prompts/NAME.md` (`NAME`: letters, digits,
dots, dashes and underscores, at most 64 characters). The text is trimmed; an empty file or one
over 64 KB is refused. Pick it in `/settings` (*system prompt*), or set `"system_prompt":
"NAME"` in the settings JSON. It applies to **new** sessions: a session keeps the text it was
created with, so editing the file does not change a running conversation or its provider cache.
A fork keeps the prompt of its source. `meno send --new|--fork --system NAME` picks one for a
single session (`--system default` is the built-in line). When the chosen file is missing the
built-in line is used, and `meno doctor` warns about it.
