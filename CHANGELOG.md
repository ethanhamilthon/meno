# Changelog

## 0.3.1

- **A `#mode` no longer deletes files, folders and skills from the message.** `#goal read @file` used to send `read` and lose the `@file` reference. The mode tags are now taken out by the new `objtag.Extract`, and every other object stays in the text. This holds for a new request, for a message queued while the agent runs, and for a started turn.
- **The system prompt sections are XML elements.** `<guidelines>`, `<subagent-role>` (subagents only), `<skills>`, `<global-instructions>`, `<project-instructions>`, `<mcp-servers>` and `<environment>` replace the `## ` headings, so the model can tell meno's text from your `AGENTS.md`. Text inside an element that contains `</` is written as `&lt;/`, so it cannot close an element early. Order and wording are unchanged. The first request after the upgrade misses the prompt cache once.
- **New status bar.** The badge on the left shows the mode and the modes of the session (`NORMAL #subagents #goal`). After it come state groups: `1x◆` (green, blinking) while the agent works, `Nx◆` (purple, blinking) for background tasks and subagents of the session, and after a `|` `Kx◆` for other running sessions and `D◉` (blue) for other sessions that finished unseen. The right side shows only the model. The goal rounds (`3/25`), the `○`/`●` dot and the `◆ n` marker are gone from the bar. On a narrow terminal the modes leave the badge first.

## 0.3.0

- **Modes changed.** `#plan` and `#build` are gone. The built-in modes are `#subagents` (the agent starts subagents and has no file, shell, web or MCP access), `#brainstorm` (only the prompt changes; it writes one spec in `.report/specs/`) and `#goal`. Pick a mode from the menu; `#none` clears them. A set of modes replaces the one before. The set stays with the session ([docs/modes.md](docs/modes.md)).
- **Typed modes are plain text.** Only modes picked from the menu count, so a typed `#goal` is plain text. The `#` menu offers modes anywhere in a message.
- **The status bar shows the modes** of the session, for example `#subagents #goal 3/25`.
- **No mode is read-only.** No mode refuses fs writes, `tasks.start` or `agents.spawn`.
- **The sandbox for `sh` is removed.** `sh` runs without a sandbox. `#subagents` has no `sh`, `fs`, `websearch` or `mcp`.
- **A goal does not nudge while subagents work.** Their events wake the session instead.
- **`subagent_max_spawn`** limits the subagents one request may start (default 50). A refused spawn returns an error to the agent.
- The session table gets a `modes` column, added on open like the other session columns (no schema version change).

## 0.2.1

- `make clean-home` deletes the dev `MENO_HOME` (`~/.meno-dev`, or `DEV_HOME`).
- **Modes.** A message that starts with `#plan`, `#build` or `#goal` (or a mode you write in `modes/*.md`) runs under that mode ([docs/modes.md](docs/modes.md)). `#plan` is read-only: the file system, tasks and subagents are closed and `sh` runs in a read-only sandbox (`sandbox-exec` on macOS, `bwrap` on Linux; without one `sh` is refused). When a plan is done meno offers to start `#build`. `#goal` keeps the agent working until it calls `goal.done` or `goal.blocked`, with a limit of rounds. The mode text reaches the model as a system event in front of the message, so the system prompt and its cache stay as they were.
- **`:login` replaces `:providers`.** One list holds API-key providers (Responses or Chat Completions) and browser logins to Claude, ChatGPT/Codex, Antigravity, xAI, Meta and Kimi, through the cliproxyapi helper that meno installs and keeps current ([docs/login.md](docs/login.md)). Entries can be switched off with `Space` and removed with `d`; a provider can be edited. With no working provider meno opens `:login` at start and keeps it open until one works.
- meno looks for the latest release on the GitHub release page first and falls back to the API, so a shared address no longer runs into the API's 60 requests an hour. Schema version 11 adds `providers.disabled`.
- The js runtime keeps its state for one answer of the agent: variables, functions and classes declared at the top level of a call are there in the next calls. A new message from the user (and a new run of a subagent) starts a clean runtime. Top-level `let`, `const` and `class` behave like `var`, so a later call can declare the same name again. Code with a top-level `return` or `await` still runs inside a function and keeps its declarations local.
- Everything about writing js (runtime rules, globals, tasks, subagents, MCP calls, `help`) moved from the system prompt into the description of the js tool. The system prompt keeps the identity, guidelines, skills, instructions, the list of MCP servers and the environment.
- One text field for every input outside the composer (forms, settings, searches, snippet rename, the agent message box): the cursor moves with the arrow keys, `Alt`/`Ctrl`+arrows jump words, `Home`/`End` and `Ctrl+A`/`Ctrl+E` go to the line edges, `Delete`, `Ctrl+W`, `Ctrl+U` and `Ctrl+K` delete, and a secret key stays editable behind its bullets. Esc drops the change to a field, Enter keeps it.
- Provider and MCP forms have no save key: a valid entry is saved the moment a field is confirmed, and an incomplete one says what is missing (`not saved: …`). `Ctrl+S` is gone.

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
