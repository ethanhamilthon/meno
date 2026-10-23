# Upgrading, data and privacy

## 0.6.9: `meno web` removed

Restart the daemon after updating.

- Removed: `meno web search`, `meno web fetch`, and the `web_search` and
  `web_allow_local` settings. Use `curl` or an MCP server. Old settings keys are
  ignored and are not written back. No data is migrated.

## 0.6.8: providers and MCP servers in JSON files, `--no-ask`

Restart the daemon after updating.

- **Providers and MCP servers live in `provider.json` and `mcp.json`** in `$MENO_HOME`.
  `/login`, `/mcp` and `meno mcp` write them; you can edit them by hand and the
  daemon sees the change. At the first start of 0.6.8 the providers and servers
  that were in `meno.db` are copied into the files (database entries first; on a
  name clash the database entry wins, as before). A file that already existed is
  copied to `provider.json.bak-0.6.7` / `mcp.json.bak-0.6.7` first. The old
  database tables stay untouched and unused, so going back to 0.6.7 works but
  loses every change made after the move. Keys are plain text in these files (or
  `env:NAME`); keep `$MENO_HOME` private. A file that is not valid JSON is
  reported and never overwritten. `theme.json` is unchanged.
- **`--no-ask`**: `meno --no-ask`, `meno send --new --no-ask` and `meno acp --no-ask`
  create sessions where `meno ask` answers with an error and the agent asks in the
  chat. The flag is stored with the session (schema version 16, additive).
- Removed: `/stop-all` and `/q!`. `/q` leaves the terminal, `/stop` and `Ctrl+C`
  stop the current turn, `meno daemon restart --force` stops everything.

## 0.6.7: simpler composer, ACP fixes

- Removed: the vim-style EDIT and VISUAL modes (`/edit`, motions, registers),
  comments (quoting the conversation into the draft) and `/drafts`. INPUT is the
  only mode. Unsent drafts are no longer written to disk. Mouse selection and
  copy stay: drag, double click and triple click copy on release.
- Old messages that contain a `<comment>` tag show it as plain text. Nothing is
  migrated, and no session data is changed.
- ACP: assistant text that came before a tool call is no longer sent a second
  time, and the command list is sent after the `session/new` and `session/load`
  responses instead of after a timer.

## 0.6.5: MCP servers that start and recover

- A stdio MCP server that answers slowly is no longer a failure. The handshake
  and the tool list wait up to 30 seconds, and a server that writes to a pipe
  gets an unbuffered stdout, so a Python server answers at once instead of
  holding its output in a buffer.
- A server definition may name its arguments and its working directory
  separately (`"command": "python3"`, `"args": ["server.py"]`, `"cwd": "/path"`),
  the usual shape of a server configuration. The `/mcp` dialog has the same
  fields. Existing definitions keep working unchanged.
- When a server cannot start, the error carries the last lines it wrote to
  stderr instead of a bare timeout.
- `meno mcp restart SERVER` reconnects a configured server without asking a
  question, so an agent can recover a server that stopped answering instead of
  waiting for a confirmation. `add` and `remove` still ask, and the question now
  ends after two minutes instead of waiting forever.
- `meno doctor` reports a server that is still starting as starting, not as
  working.
- Daemon protocol is **7**. Update both client and daemon so they agree on
  protocol 7. Existing sessions and settings are preserved; the database gains
  two columns for the new server fields.

## 0.6.4: session picker metadata and freshness

- The sessions picker dialog shows accurate turn counts, model selection details,
  timestamps, and live attention/activity for persisted sessions without loading
  their transcripts.
- Live catalog freshness: turn publications, compaction/history replacement,
  renames, and deletions invalidate the global catalog. The dialog reflects changes
  made by other clients or background sessions without closing and reopening.
- UI catalog consolidation: the picker and global status share a single recoverable
  event-driven cache (`globalSessions`), eliminating duplicate requests and
  stale-response races.
- Attached runtime precedence: active runs and local drafts remain authoritatively
  projected; global catalog hydration never overwrites in-flight runtime projections.
- Daemon protocol is **6**. Update both client and daemon so they agree on protocol 6.
  Existing sessions and settings are preserved without database migration.

## 0.6.3: daemon/client state and streaming efficiency

- Streaming text, reasoning and command fragments no longer rewrite session history
  in SQLite or trigger global-status reads/events. Completed boundaries remain durable.
- Session metadata and tasks update through ordered events instead of repeated
  snapshots. Snapshots still recover missing events, daemon restarts and compaction.
- One client projection owns session state; background sessions do not borrow the
  active composer. Global counters use a recoverable event subscription, not polling.
- Configuration refreshes use one bundled request; existing configuration routes,
  provider-key redaction and per-window overrides are retained.
- Daemon protocol is **5**. Finish active work, close old windows, update, and
  reopen meno so client and daemon use the same protocol. Existing session data
  and settings are preserved; no database migration or reset is required.

## 0.6.2: timeline, steering, request export and safe updates

- Timeline text shares a common left anchor: command titles, source, output,
  messages, reasoning, image captions, notices and run summaries. Long tool rows
  wrap inside the gutters without losing text; Markdown/source indentation stays.
- Steering consumption and live-response boundaries stay consistent across events,
  snapshots and reconnects. Replayed turns are identified by ID, not by their text.
- `/export [path]` includes recorded system prompts, tool definitions and per-attempt
  model/effort/token accounting, including retries and compaction. Older sessions
  explicitly mark unavailable details. Prompts can contain private project content;
  review exports before sharing. Provider credentials are not exported.
- Daemon protocol is **4**. Before updating from 0.6.1 or older, finish active work,
  close meno windows, stop the old daemon, update, then launch meno again.
- Request configurations are stored additively in the existing database. Historical
  instructions cannot be reconstructed for requests made before this release.

### Updating the executable and daemon

With an updater that includes the automatic-stop lifecycle, `meno update`
checks the release, downloads and verifies the executable, then asks the daemon
for an atomic idle-only shutdown. Active runs, queued work or background tasks
block the update: finish or stop them and retry. There is no update force flag.
A failed download or verification leaves the running daemon untouched. A failed
shutdown prevents installation. No available update leaves the runtime unchanged.

A successful update leaves the daemon **stopped**; the updater does not restart
it. Close already-open meno windows and launch meno again to start the new daemon
from the installed executable. An old UI is not upgraded in place. Concurrent
startup is serialized with installation, and an old client that detects a
replaced executable asks you to reopen it instead of restarting itself.
The selected `MENO_HOME` determines which daemon is stopped; sessions, settings,
credentials and stored results are preserved.

If replacement fails after shutdown, the old executable is retained and the
daemon stays stopped. Reopen meno to start it again; there is no automatic
restart or claimed rollback. If shutdown or cleanup cannot be confirmed, the
command returns an error rather than reporting a successful update.

An older installed updater cannot gain this behavior retroactively. For an old
updater, finish work, close all meno windows, run `meno daemon stop`, update, and
launch meno again. A running daemon without the atomic idle-shutdown endpoint is
refused by the new updater with the same manual instructions. Protocol 4 also
prevents clients sending `origin` to the strict protocol-3 request decoder.

## 0.6.1: sessions are stored with their first message

Restart the daemon after updating (the protocol is unchanged). A new session
lives in the daemon's memory until its first message; nothing is written before.
Empty sessions that older versions stored stay in the database but are no longer
listed. Nothing is deleted or migrated.

## 0.6.0: shared sessions, drafts, quotes and web access

Restart the daemon after updating: the protocol is now 3, and an old daemon is
refused with `run meno daemon restart`. Nothing in your data is reset. The
database, session files, keys, provider and MCP overlays and stored MCP results
are read as before; new data is only added.

- **Several windows.** History, the queue, runs, tasks, providers and MCP are
  shared by every window. The draft, cursor, scroll position and selected session
  are yours alone. Snapshots carry a version, so a late refresh can never roll a
  window back, and a turn is never shown twice or lost when a window attaches in
  the middle of a run. See [sessions](sessions.md).
- **Leaving.** `/q` (and the old `/q!`) only leaves this terminal; runs and
  tasks go on. `/stop` stops the current turn. `/stop-all` stops every running
  turn and asks you to repeat it within ten seconds. On the first-run screen
  Ctrl+C quits.
- **Drafts survive.** An unsent draft is saved under `$MENO_HOME/drafts/`
  (private files, one per window and session, images stored next to them). After
  a crash or restart meno starts a NEW session as before and tells you that
  drafts exist; `/drafts` puts them in the composer. Undo now brings deleted
  images back.
- **Settings.** The settings dialog sends only the fields you changed. A field
  someone else changed first is reported as a conflict and reloaded instead of
  being overwritten. A provider is only deleted when you delete it.
- **Comments.** Select text in the conversation (EDIT, `v`) and press Enter or
  `c` to attach it as a quote. See [comments](comments.md).
- **Web.** `meno web search` and `meno web fetch`, see [web](web.md), and
  `meno doctor` for a read-only health check, see [cli](cli.md).
- **Prompt.** The working rules (editing, verification, output) now live once in
  the system prompt; the bash tool description only describes the tool.
- **Skills.** A skill with `disable-model-invocation: true` is not offered to the
  model; it works only through an explicit `/skill:name`.

Rollback: the new files (`drafts/`) are plain additions and an older meno ignores
them. Do not restore an older `meno.db` over a live database.

## 0.5.5: slash commands and padded input

Commands now use `/` instead of `:`, for example `/edit`, `/model` and `/login`.
Skills use `/skill:<name>`; `/skills` still manages skills. Old colon commands
and bare `/name` skill references are ordinary text. Saved skill objects remain
valid, and no database migration is needed. Restart meno after updating.

The dark-gray input has one padding row above and below its text. Activity is a
subtle green wave on the input background. The unruled footer shows the session
on the left, global counts in the center, and model/effort/context on the right.
Copying a history selection now returns the cursor to the input correctly.

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
