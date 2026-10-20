# Upgrading, data and privacy

## 0.5.4: shared MCP runtime and compact footer

MCP configuration and agent calls now share one runtime. Restart the daemon after
upgrading so existing sessions use it. Tool calls return a JSON envelope, including
`isError`, `resultId`, structured data and artifact paths. Tool-reported errors no
longer fail the CLI command. Existing empty stored results cannot be recovered.
MCP administration asks for approval. Env/headers use lossless JSON-object editing.

The footer shows `model | effort`, gray daemon counts, and right-aligned context.
Mode/dialog badges and bottom composer padding are removed. Esc in INPUT preserves
the draft; use `:edit` to enter EDIT. Header and footer share the activity wave.
Words wrap without dropping source text. Partial Bash arguments stay hidden until
execution starts. Identical frames no longer repeat terminal repaint sequences.
No database reset or migration is required.

## 0.5.2: commands inside the input

COMMAND mode is gone: type `:` at the start of a word in INPUT and pick a command
(see [UI](ui.md)). Esc in INPUT enters EDIT. The global `$MENO_HOME/AGENTS.md` is no
longer read; put shared rules in a project `AGENTS.md` or a skill. The system prompt
is now only the identity line, skills, the project `AGENTS.md`, MCP servers and the
environment; the rules for using bash moved into the `bash` tool description. The
reply-style rules are gone. Bash commands are colored by role, the run summary reads
`↑in ↓out · cache N% · cost`, and the empty composer has three fixed hints.

## 0.5.1: one theme, `:edit`

`meno`, `pure-black` and `catppuccin-mocha` are gone; the only built-in theme is `default` (the old pure-black palette). Saved names of removed themes fall back to `default`; add others in `theme.json`. `:v` and `:visual` are replaced by `:edit`, which navigates and edits like vim normal mode; `v` starts a selection. Zero counts leave the status bar, which also shows `ctx/max`.

## 0.5.0: clean database and simplified interface

Stop all older meno daemons before the first launch. 0.5.0 creates a verified,
private SQLite backup in `$MENO_HOME/backups/`, then starts with a clean database.
History and DB-backed accounts/settings are not transferred; onboarding may be
needed. JSON overlays, skills, projects and old session files remain untouched.
A backup or integrity failure stops startup instead of erasing the original.
The reset happens once; later launches keep your new data.

Backups may contain API keys. To recover, stop every daemon, preserve the new DB
separately, restore the selected backup as `meno.db`, and use the previous binary.
Do not restore over a live database. There is no automatic downgrade/import path.

Application `#` modes, `meno goal` and snippets are removed. INPUT replaces INSERT;
NORMAL is gone. Esc toggles INPUT/COMMAND and `:v` opens VISUAL. `:context` holds
usage metrics; the bar shows running/tasks/Asking/unread done counts. See [UI](ui.md).

## 0.4.1: stable history, no execution timeouts, parallel bash calls

The settings `bash timeout`, `bash strings` and `bash string lines` are gone; old settings files that hold them still load and the keys disappear on the next save. A bash call or a task runs until it ends, you stop it, or it is cancelled. `meno tasks start --timeout` now fails; `meno tasks wait --timeout` is unchanged. Old turns that timed out still show as they did.

Already-sent tool results are no longer shortened on later requests, preserving provider prefix-cache reuse. The 56 KB source output cap and automatic compaction are unchanged. No data migration is needed; existing stored outputs are used as-is. The Bash prompt now makes clear that each call starts in the session working directory, so a redundant `cd` is unnecessary.

## 0.4.0: bash tool and daemon

The `js` tool and its JavaScript runtime are gone. The model has one `bash` tool and the `meno` command (see [the agent's tool](tools.md)). The TUI is a client of the background daemon, which starts when you start meno and keeps running after you close the terminal; `meno daemon stop` ends it. Old sessions open as before: their `js` calls are sent to the model as plain text ("ran js: …") and are never run again. Schema version 12 adds `sessions.tool`; the column is kept for old rows and every new session uses `bash`. The `js_*` settings are read as aliases of `bash_*` and are removed from the settings file on its next write. The `tool` setting no longer selects anything. `meno agents` and the `:agents` dialog are gone.

## 0.3.1

- No data or schema change. The system prompt is built again on start, so every session sends the new XML-style prompt from its next request; the prompt cache is cold for that one request.
- Text in your `AGENTS.md` or skill descriptions that contains `</` reaches the model as `&lt;/`.
- The status bar changed (state groups `Nx◆`, modes in the badge; see [ui](ui.md#status-bar)). The goal round counter is not shown there any more.

## 0.3.0

- Modes changed. `#plan` and `#build` are gone. No mode is read-only any more: none refuses fs writes, `tasks.start`. Built-in modes: `#brainstorm`, `#goal`, picked from the menu; `#none` clears them. A typed `#goal` is plain text now. Sessions saved by 0.2.1 start with no modes; a mode name saved in a session that no longer exists is ignored.
- Your own `modes/*.md` files, in `$MENO_HOME/modes` and in `.meno/modes` of a project, are no longer read. You can delete them.
- The sandbox for `sh` is removed. `sh` runs without a sandbox.
- The sessions table gets a `modes` column when meno opens `meno.db`. The schema version does not change.

## Earlier upgrades

Upgrade by running the installer again (see the README). Your data in `$MENO_HOME` is kept.

The one-time import of `settings.json`, `providers.json`, `skills.json` and
`model-catalog.json` into `meno.db` is gone; those files are no longer read and can be
deleted. `mcp.json` now means something else: it adds MCP servers (see
[settings](settings.md)), so an old `mcp.json` is read as that list.

Before 0.5.0, migrations were additive and ran when meno opened `meno.db`. Sessions from older
versions keep all their turns; their history IDs are assigned on first load.
Sessions compacted by an old version cannot get back the turns that version
discarded: they are marked as partially covered and forks of them are
conversation-only. Version journals start at the upgrade.

The `state` global and the `sessions` global are gone, and so is `session.compact`: the agent cannot compact by itself. Context is compacted at `compact_percent` (set it in `:settings`, "auto compact") and with `:compact`. Schema version 11 adds the `disabled` column to `providers` (additive): a provider can be switched off in `:login`. The `:providers` dialog is gone; `:login` replaces it. Schema version 8 adds the `run` and `models_json` columns to `providers` (additive). Schema version 7 deletes the saved `state` values of every session; plans are kept.

**Tool API changes in an earlier release** (the API was pre-release): `:reset` is gone;
ask takes a form, not a string; MCP calls return an envelope instead of text and no longer
fail on a tool's own error (check `isError`).

Data stays on your machine, in `$MENO_HOME`. API keys are stored in plain text in
`meno.db` (mode 0600). Default diagnostics contain request metadata and token metrics,
never message text, image bytes or keys. Nothing is sent anywhere except to the
providers and MCP servers you configure. SIGKILL cannot run cleanup: a task started by
a killed meno may keep running; meno never kills a process by PID alone.
