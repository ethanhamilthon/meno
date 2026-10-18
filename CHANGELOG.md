# Changelog

## 0.2.0

- `websearch(query, {count?})` replaces `fetch`. It asks the provider's native web search first, then Brave or Tavily (Web tab: backend and key), then the keyless DuckDuckGo fallback; `count` is 1..10.
- The js runtime no longer has `fetch`, `URL`, `URLSearchParams` or the `env` global. For another HTTP API use `sh(curl ...)`. Background tasks still take `env`, and a name that appears twice now replaces the old value instead of stacking.
- New host helpers: `hash` (sha256, sha1, md5, also for files), `path` (join, dirname, basename, resolve and the rest) and `base64`/`btoa`/`atob`, all UTF-8 safe.
- `help("sh")`, `help("view")` and `help("websearch")` reach their topics through aliases, and an unknown topic lists the aliases along with the topic names.
- A batched script that redeclares a top-level `let` or `const` no longer fails with "identifier has already been declared": the parser turns those declarations into `var`.
- `fs.write` refuses to replace an existing file unless it gets `{overwrite: true}`; the error points to `fs.patch` and `fs.edit`. `fs.patch` reports the line where the old text stopped matching, with what the file has there and what the patch expected.
- Settings are split into tabs: General, Display, Web, Notify, Agents and Context (`Tab`/`]` and `Shift+Tab`/`[`).
- `:cache`, `:notify`, `:attach` and `:theme` are gone. The theme is a setting with a live preview, images come in by paste, drag and drop, or an image picked with `@`, and the opt-in payload log stays behind `MENO_DEBUG_PAYLOADS=1`.
- Notify volume (20 to 100 percent) scales the built-in sounds and previews on change; it is ignored for a custom `notify_command`.
- Snippets: `:save_snippet <name>` stores the selection from the composer or the transcript, `:paste_snippet <name>` inserts it at the cursor, and `:snippets` lists, renames and deletes. They live in `meno.db` (new additive migration).
- Defaults for subagents: `subagent_provider`, `subagent_model` and `subagent_effort` (Agents tab). `agents.spawn` falls back field by field: the value it was given, then the setting, then the calling agent's value.
- askUser forms lost the separate Submit button: `Enter` selects and advances, and `Enter` on the last question sends every answer at once. Going back with the arrow keys or `Shift+Tab` keeps what was filled in.
- The welcome wordmark scrambles the glyph of every lit pixel while the wave moves behind it.
- The system prompt and the js tool description now name `websearch` instead of `fetch`, `URL` and `env`.

## 0.1.2

- `meno update` installs the latest GitHub release over the running binary (SHA-256 checked). `meno -v` and `meno -h` are new; any other unknown command prints an error, points to `meno -h` and exits 1. Only a bare `meno` opens the UI.
- At start meno looks for a newer release (at most every 6 hours; `MENO_NO_UPDATE_CHECK=1` turns it off) and shows it on the welcome screen and as a notice.
- Every js call carries a `title` (up to 6 words) written by the model. It sits in the bottom right corner of the code block. The composer shows the latest title instead of `thinking...`; `thinking...` stays only until the first call of a run, and `writing...` shows while the model streams.
- `make run` uses its own `MENO_HOME` (`~/.meno-dev`), so a dev build does not mix with an installed one.
- Fixes for flaky CI tests: `tasks.wait` could return a stale `running` status for a task that had just finished.

## 0.1.1

- Single version source: `internal/version/VERSION`; the CLI, welcome screen and help title all read it.
- The agent plan (`todo.*`, `:todo`, the plan block and its reminders) is removed.
- `session.rename` is removed. A session is named after your first message; rename it with `:rename <name>`.
- The timing of a js call is a small badge in the top right corner of its block: `⠹ 2.3s/120s` while running, `✓ 2.3s` when done, `✕ 2.3s` on error. The footer line with the progress bar is gone.

## 0.1.0 — first release

Everything below is new relative to the pre-release builds.

- Fresh JavaScript runtime per call, with durable `state`, strict result rendering,
  `fs.read` ranges, `fs.inspect`, `fs.search`, hash-guarded `fs.patch`.
- Complete session history that survives compaction, `:fork`, versioned state and plan,
  automatic compaction safety net, `sessions.list/search/read`.
- One steering queue for your messages, subagent and task events; request-scoped `:stop`.
- Multi-question `askUser` forms, an agent plan with acceptance criteria.
- Background tasks (`tasks.*`, `:tasks`) and subagents (`agents.*`, `:agents`).
- Typed MCP results with a local artifact store.
- JavaScript highlighting for completed tool calls and string folding.
- Terminal graphics (Kitty, Sixel, iTerm) — see docs/graphics.md for what was verified.
- `js strings` setting in `:settings` (folds long string literals in tool-call source); the `:strings` command is gone.
- The plan block keeps its place while a reply streams, so the timeline no longer jumps.
- The system prompt states that the js runtime is synchronous (no async/await) and that the agent always names its session.
- MIT license, CI, release script with checksums.

Breaking (pre-release API): `:reset` removed; `askUser` signature; MCP envelope; see
docs/upgrading.md.
