# Upgrading, data and privacy

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

The `state` global and the `sessions` global are gone, and so is `session.compact`: the agent cannot compact by itself. Context is compacted at `compact_percent` (set it in `:settings`, "auto compact") and with `:compact`. Schema version 8 adds the `run` and `models_json` columns to `providers` (additive). Schema version 7 deletes the saved `state` values of every session; plans are kept.

**Tool API changes in this release** (the API was pre-release): `:reset` is gone and each
js call is a fresh runtime; `askUser` takes a form, not a string;
MCP calls return an envelope instead of text and no longer throw on a tool's own
error (check `isError`); `await` is limited to settled values.

Data stays on your machine, in `$MENO_HOME`. API keys are stored in plain text in
`meno.db` (mode 0600). Default diagnostics contain request metadata and token metrics,
never message text, image bytes or keys. Nothing is sent anywhere except to the
providers and MCP servers you configure. SIGKILL cannot run cleanup: a task started by
a killed meno may keep running; meno never kills a process by PID alone.
