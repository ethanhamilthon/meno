# Changelog

## 0.6.11

Restart the daemon after updating. No data is migrated. `mcp.json` gains an optional `protocol` field.

- **MCP, modern protocol (2026-07-28), tools only.** By default (`protocol` empty or `auto`) meno probes a server with
  `server/discover` first and falls back to the legacy `initialize` handshake. `legacy` and `modern` pin one era. Supported:
  per-request `_meta`, `Mcp-Method`/`Mcp-Name` and `x-mcp-header` request headers, `resultType`. Not supported: `subscriptions/listen`,
  list-changed refresh, caching, multi round-trip input (`input_required` is reported as an error), OAuth, resources and prompts.
  It has been tested against in-repo fake servers (stdio and HTTP) only, not against a real modern server.
- **Auto fallback limits.** On HTTP only a 400/404/405 or a non-modern answer means "legacy"; 401, 5xx and network errors are real errors.
  A legacy stdio server that stays silent is started twice and can delay startup by up to 10 seconds: set `"protocol": "legacy"`.
  The `sse` transport is always legacy.
- **Legacy MCP:** a forgotten HTTP session (404) is restarted once; a server without the `tools` capability starts without
  tools; `title`, `outputSchema`, `annotations` and `_meta` of tools are kept (annotations are hints, never permission).
- **Skills:** `.agents/skills` of the project and of your home directory are read as well (not with a custom `MENO_HOME`);
  `SKILL.md` frontmatter is parsed as YAML with a lenient fallback; a skill with problems still loads and `meno doctor` lists
  warnings; skill paths in the prompt are relative where possible; after compaction the summary lists skill files that were read.
  `allowed-tools`, `license` and `compatibility` are not modeled.

## 0.6.10

Restart the daemon after updating. The database schema goes to version 17 (additive).
See [docs/upgrading.md](docs/upgrading.md) for the one behavior change.

- **`meno send --fork SESSION [--at N]`** creates a session from the history of another and sends a
  message to it. The session must be named: `send` without `-s`, `--new` or `--fork` is a usage error and no
  longer writes to `$MENO_SESSION`.
- **Choose the model and the setting of a session:** `--model PROVIDER/MODEL`, `--effort`, `--cwd`, `--title`,
  `--system` for `--new` and `--fork`; `--model` and `--effort` also change an idle session with `-s`. The daemon
  checks them before anything is created.
- **`send` for agents:** `--timeout`, a JSON result with the session id, status, usage and warnings, a question of the
  agent ends the wait with exit 5, a stopped run gives exit 6, and an interrupted `send` cancels the run it created
  (`--keep-running`, `--cancel-on-exit`). Background work is `meno tasks start "meno send ..."`.
- **`meno sessions`:** `--status`, `--spawned-by`, `--title-prefix`, `--limit`, and `show`, `cancel` and `answer`.
  Sessions record which session started them; `max_spawn_depth` and `max_workers` limit that (exit 3).
- **`meno tail`** streams while the session runs, follows several sessions or `--children-of`, filters with
  `--events` and continues with `--from`.
- **Custom system prompts.** Files in `~/.meno/system-prompts/NAME.md` replace the identity line; pick one in
  `/settings`, with the `system_prompt` setting, or per session. Without one the built-in line is used. `meno doctor`
  checks the file.
- The tool description of the model lists the new `send`, `sessions` and `tail` forms.

## 0.6.9

Restart the daemon after updating. No data is migrated.

- **Removed** `meno web search` and `meno web fetch`, and the `web_search` and
  `web_allow_local` settings. Use `curl` or an MCP server. `meno web` is now an
  ordinary unknown command. Old settings files that still have the two keys load
  fine and drop them at the next save.

## 0.6.8

Restart the daemon after updating. Providers and MCP servers move from the
database to `provider.json` and `mcp.json` at the first start (a backup of an
existing file is kept as `*.bak-0.6.7`); see [docs/upgrading.md](docs/upgrading.md).

- **JSON configuration.** `provider.json` and `mcp.json` are now the stored
  configuration: the dialogs and `meno mcp` edit them, so do you. A broken file is
  reported and never overwritten.
- **`--no-ask`** for `meno`, `meno send --new` and `meno acp`: sessions without
  `meno ask`; the agent is told to ask in the chat.
- **Removed** `/stop-all` and `/q!`.

## 0.6.6

Restart the daemon after updating. No data is deleted or migrated; the daemon
protocol is unchanged.

- **`meno acp`.** Use Meno as an agent in any Agent Client Protocol client: chat,
  tool cards, cancel, create/list/load, models and effort, usage and titles,
  exposed thinking, images, skills and slash commands. Questions from `meno ask`
  appear as chat messages and are answered with `/answer`; an open question
  survives a reconnect. See [docs/acp.md](docs/acp.md) for the limits (no injected
  MCP servers, no permission prompts, no plan updates yet).
- **Session consent.** The bash tool description asks for explicit consent before
  an agent creates or messages other sessions.
- **Internal consolidation.** Packages were merged and renamed (`tui`, `render`,
  `content`, `process`, `api`, `configsvc`), the file size limit is now 500 lines
  preferred and 700 at most. No behavior, wire format or data changed.

## 0.6.1

Restart the daemon after updating. No data is deleted or migrated.

- **No empty sessions.** A session is created in the daemon's memory and stored
  only when its first message is sent. Opening meno or `/new` no longer leaves
  empty entries in `/sessions` or `meno sessions`. Empty entries stored by older
  versions are hidden, not deleted. If the daemon restarts before the first
  message, the typed text is sent in a fresh session.
- **The name appears at once.** A session is named when its first message is
  sent, not when the run ends.
- **Skills after the slash.** `/pla` finds the skill `plan`; Enter or Tab inserts it
  as one object and never runs or sends. A command with the same name wins.
- **Status bar.** Counters keep their colors on narrow terminals and a click opens
  `/sessions` with that filter; `Tab` in `/sessions` cycles all, running, asking,
  done. Failures to send stay on screen until Esc.
- **Smoother wave.** The running pulse now rises and falls smoothly and wraps around
  without a jump.
- **Per-window settings.** Theme and notification settings can be kept for one
  window (`s` in `/settings`); `r` returns to the shared defaults.
- **Origin.** A run records which window started it; other windows say so.
- **`meno ask` form.** The bash tool description and `meno help ask` now show the
  JSON form and the answer shape.

## 0.6.0

Restart the daemon after updating (protocol 3). No data is reset or migrated.

- **Several windows stay consistent.** A snapshot now carries a version and the
  cursor of the state it was read with; the client drops older snapshots, replayed
  turns and the late end of an old run. Sessions that are created, forked or
  deleted reach every window. Queue recalls and the last run outcome are shared.
- **Fixed lost work.** A provider error is no longer reported as done; a stream
  consumer's error is returned instead of reconnecting; a closed stream is an
  interruption, not success. Compaction keeps messages that arrive while it runs
  and a second compaction is refused. Images sent during a run reach the model.
  Two clients answering one question can no longer both win.
- **Settings and providers.** Settings are saved as field patches with conflict
  detection; a provider is deleted only on an explicit delete.
- **Drafts.** Unsent drafts and their images are saved per window and recovered with
  `/drafts` after a crash. Undo restores deleted images.
- **Leaving.** `/q` and `/q!` only detach; `/stop-all` stops every turn after a
  repeated confirmation; Ctrl+C quits the first-run screen. A broken stream shows
  `offline` and keeps your draft.
- **Comments.** Quote saved assistant text, code, output and errors into a message
  with Enter or `c` in a selection. See docs/comments.md.
- **`meno web` returns, rebuilt.** `meno web search|fetch` runs in the daemon under an
  address policy (no private, loopback or metadata destinations unless allowed), with
  size, time and redirect limits. Search uses the one provider you set in `web_search`;
  it is never chosen for you. This is a new implementation: the old `websearch`
  settings and Web tab stay removed.
- **`meno doctor`** is a read-only health check with suggested fixes.
- **Interface.** Model effort is a preview until Enter; dialogs use nearly the full
  width on narrow terminals; task logs scroll, page, wrap and copy; a pending question
  can be left with Ctrl+O without answering it.
- **Prompt.** Working rules moved into the system prompt; the bash description only
  describes the tool. Skills with `disable-model-invocation: true` are manual only.
- **Process logs.** The disk budget now counts escaped bytes; admission of new
  processes is atomic with shutdown; a silent event stream is closed and resumed.
- **Gates.** Per-package coverage floors (`make cover`) and a relative benchmark
  comparison (`make bench-compare`).

## 0.5.5

- Use slash commands (`/edit`, `/model`, `/login`) and namespaced skill references
  (`/skill:<name>`), with distinct command and skill completion.
- Add a dark-gray input surface with top and bottom padding and a subtle green
  background activity wave. Remove animated separator rules.
- Unify footer metadata: session left, daemon counts center, model/effort/context right.
- Fix history selection copy returning to INPUT while leaving the cursor in history.
- Prevent task continuations from starting a run after engine shutdown begins.
- Fix declared short flags such as `meno send -s`, preserving literal shell flags
  and the `--` argument separator.

## Unreleased

- **Removed the file commands.** `meno read`, `edit`, `write` and `search` and the `files` group are gone; the agent uses `cat`, `sed`, heredocs and `rg` or `grep`.
- **Removed web search.** `meno web`, the Web settings tab and the `websearch` and `websearch_key` settings are gone; the agent reaches the web with `curl` or an MCP server.
- **Removed effort routing.** The Routing settings tab and the `effort_router*` settings are gone; a run keeps the effort of its session. Old settings files that still hold the removed keys load, and the keys disappear on the next save.
- **Fixed the error `provider "" is not configured`.** The UI and the engine now read one settings source (`settings.json` first, SQLite only when the file is absent). A session with no provider and model takes the current defaults before its next message; a partial pair or a removed provider is reported and never replaced silently.
- **Bash calls run in parallel and have no time limit.** All calls of one model response start together; results keep call order and a failing call does not stop the others. The `bash timeout`, `bash strings` and `bash string lines` settings, the model `timeout` argument and `meno tasks start --timeout` (now an error) are removed.
- **Your message moves running calls to background tasks.** The model gets one result per call with the task ID and answers at once; task notices and goal events never interrupt a call. Stop cancels foreground calls only: tasks, started or moved, keep running.
- Removed dead code and trimmed the real-terminal suite and the benchmarks.

## 0.4.0

- **The agent has one `bash` tool; the JavaScript runtime is gone.** The `js` tool, goja and `internal/jsvm` are removed. Each call runs one command in a fresh `/bin/sh`, and the `meno` command inside it gives the agent files (`read`, `edit`, `write`, `search`), `mcp`, `tasks`, `ask`, `goal`, `web`, `view` and `models`. Old `js` turns are sent to the model as plain text. See [the agent's tool](docs/tools.md).
- **A background daemon owns the work; the TUI is a thin client.** Sessions, runs, tasks, MCP servers, logins and configuration live in the daemon (unix socket `$MENO_HOME/run/meno.sock`, started on demand). Closing the TUI does not stop a run, several clients can use one session, and a killed daemon recovers. See [the daemon](docs/daemon.md).
- New commands for scripts and other agents: `meno sessions`, `meno send`, `meno tail` and `meno daemon run|start|stop|restart|status|logs`. See [the command line](docs/cli.md).
- Removed: `meno agents`, the `:agents` dialog and the `#subagents` mode, the `js_*` settings keys (they still load as `bash_*` aliases), and the `tool` setting as a switch. The bash timeout is seconds, `off` or `model`.
- Add daemon-owned configuration resources, sequenced session events with resume, ephemeral session tokens for agent calls, steering and queue recall through the daemon, and world-change events for edited instructions.
- Add `meno` command help embedded in the binary (`meno <group> --help`) and `--json` output.

### Other changes

- Complete the JavaScript-to-bash settings rename. Legacy `js_*` keys still load as aliases, but saved settings use only `bash_*` keys; the bash timeout is seconds, `off`, or `model`.

- Add a SQLite-backed daemon engine with persistent session runs, steering and stop, ask pause/resume, retries and goal modes, subagents, tasks and MCP services. The daemon emits sequenced session events and enforces ephemeral session/mode tokens for agent resource routes. `meno daemon run` now starts this backend.
- Add daemon-owned configuration resources for settings, providers, OAuth/cliproxy login, skills, snippets, MCP servers, provider models, file search, and debug requests; add typed config clients and config change notifications.
- Prepare bash-only migration: add mode command-group permissions, bash settings with legacy JS aliases, persisted session tool selection, bash prompt/schema text, and plain-text replay conversion for old JS turns.

- **The composer is transparent and identifies the session.** Its top row is a `─` rule with the session name at the right, clipped to 30% of the terminal width; untitled sessions show their working-directory name (or `~` in the home directory).
- Trim answered tool output above 1 KB while preserving the stored result.
- Rebuild the system prompt when mode policy changes; add gated voice, method, editing, verification and output sections.
- Keep cliproxyapi running while disabled OAuth accounts exist. meno now discovers, starts and restarts the helper, retries failed account actions once, and repairs port conflicts. The `:login` list shows helper errors, account status notes and a manual restart action.
- Added optional per-request effort routing. The router never changes the model, needs two matching verdicts to change effort, and shows each applied change. Configure it in `:settings` under **Agents**.

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
