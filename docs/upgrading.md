# Upgrading, data and privacy

## Bash migration preparation

Schema version 12 adds `sessions.tool`: existing rows default to `js`; sessions created by the daemon migration use `bash`. New sessions default to bash. The old `js_*` settings remain readable as aliases and are removed from the settings store on its next write. JS turns in a JS session can be converted to plain text for a bash request; the in-process loop is not switched in this stage.

## 0.3.1

- No data or schema change. The system prompt is built again on start, so every session sends the new XML-style prompt from its next request; the prompt cache is cold for that one request.
- Text in your `AGENTS.md` or skill descriptions that contains `</` reaches the model as `&lt;/`.
- The status bar changed (state groups `Nx◆`, modes in the badge; see [ui](ui.md#status-bar)). The goal round counter is not shown there any more.

## 0.3.0

- Modes changed. `#plan` and `#build` are gone. No mode is read-only any more: none refuses fs writes, `tasks.start` or `agents.spawn`. Built-in modes: `#subagents`, `#brainstorm`, `#goal`, picked from the menu; `#none` clears them. A typed `#goal` is plain text now. Sessions saved by 0.2.1 start with no modes; a mode name saved in a session that no longer exists is ignored.
- Your own `modes/*.md` files, in `$MENO_HOME/modes` and in `.meno/modes` of a project, are no longer read. You can delete them.
- The sandbox for `sh` is removed. `sh` runs without a sandbox. `#subagents` has no `sh`, `fs`, `websearch` or `mcp`.
- `subagent_max_spawn` (new, default 50) limits the subagents one request may start. It has no row in `:settings`.
- A goal does not nudge while its subagents work; their events wake it.
- The sessions table gets a `modes` column when meno opens `meno.db`. The schema version does not change.

## Earlier upgrades

Upgrade by running the installer again (see the README). Your data in `$MENO_HOME` is kept.

The one-time import of `settings.json`, `providers.json`, `skills.json` and
`model-catalog.json` into `meno.db` is gone; those files are no longer read and can be
deleted. `mcp.json` now means something else: it adds MCP servers (see
[settings](settings.md)), so an old `mcp.json` is read as that list.

Migrations are additive and run when meno opens `meno.db`. Sessions from older
versions keep all their turns; their history IDs are assigned on first load.
Sessions compacted by an old version cannot get back the turns that version
discarded: they are marked as partially covered and forks of them are
conversation-only. Version journals start at the upgrade.

The `state` global and the `sessions` global are gone, and so is `session.compact`: the agent cannot compact by itself. Context is compacted at `compact_percent` (set it in `:settings`, "auto compact") and with `:compact`. Schema version 11 adds the `disabled` column to `providers` (additive): a provider can be switched off in `:login`. The `:providers` dialog is gone; `:login` replaces it. Schema version 8 adds the `run` and `models_json` columns to `providers` (additive). Schema version 7 deletes the saved `state` values of every session; plans are kept.

**Tool API changes in this release** (the API was pre-release): `:reset` is gone and the js
runtime lasts for one answer of the agent; `askUser` takes a form, not a string;
MCP calls return an envelope instead of text and no longer throw on a tool's own
error (check `isError`); `await` is limited to settled values.

Data stays on your machine, in `$MENO_HOME`. API keys are stored in plain text in
`meno.db` (mode 0600). Default diagnostics contain request metadata and token metrics,
never message text, image bytes or keys. Nothing is sent anywhere except to the
providers and MCP servers you configure. SIGKILL cannot run cleanup: a task started by
a killed meno may keep running; meno never kills a process by PID alone.
