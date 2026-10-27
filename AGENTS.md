# Repository instructions

## File size

- Keep files small and focused: aim for 500 lines or fewer, with a hard maximum
  of 700 lines for hand-written Go source and tests. Documentation stays concise.
- Split files by responsibility before they exceed the limit. Do not compress
  code or remove useful explanations just to reduce the line count.
- When changing an existing oversized file, extract the affected responsibility
  into smaller files rather than adding more to it. Avoid unrelated rewrites.
- Generated files and dependency lock/checksum files are exempt; do not edit
  generated output manually.

## Language

- Write all UI text, code, comments, identifiers, error messages, tests, and
  documentation in English.

## Separation of responsibilities

- Keep UI, agent orchestration, external APIs, tool execution, and persistence
  in separate packages and focused files. Do not mix these layers for convenience.
- Keep rendering in `internal/render` and application interaction/wiring in
  `internal/tui`; do not put provider HTTP calls or database queries in views.
- Keep the agent loop independent of terminal rendering. Pass events and data
  through explicit interfaces instead of reaching into UI state.
- Keep provider API/protocol code in `internal/llm`, MCP transport code in
  `internal/mcp`, and Bash tool execution in `internal/process`.
- Keep SQL/schema concerns in `internal/db`, configuration persistence in
  `internal/config`, session persistence in the agent's session storage files,
  and reusable JSON file persistence in `internal/store`.
- Extend the existing responsible package before creating a parallel abstraction.

## Repo map

meno is a Go terminal coding agent with a tcell UI, a daemon-owned engine,
Bash tools, provider APIs, MCP integrations, and SQLite persistence.

| Path | Responsibility / starting points |
| --- | --- |
| `cmd/meno/main.go` | CLI entry point; terminal setup and application lifecycle. Update orchestration: `cmd/meno/update.go`. ACP entry: `cmd/meno/acp.go`. |
| `internal/tui/` | TUI controller, state, input, commands, composer-anchored panels, session/runtime wiring. Panels: `panel.go`, `popup.go`; cached transcript: `transcript_cache.go`, `linecache.go`; frame deadlines: `frame_schedule.go`; live palette reload: `theme_reload.go`; static welcome: `welcome_header.go`. Start with `app.go`, `state.go`, `runtime.go`, `turn.go`. Mouse selection and copy: `selection.go`, `mouse.go`, `doc.go` (document positions), `textdoc.go`; composer undo: `draft_undo.go`. Stream state: `connection.go`; immutable session updates: `wire.go`, `projection_effects.go`; global subscription/hydration: `global_watch.go`. Text fields outside the composer: `textedit*.go` (`textEdit`), forms in `forms.go`. Slash completion of skills: `slash_completion.go`; status-bar segments and clicks: `status_segs.go`, `attention_filter.go`; per-window setting overrides: `local_preferences.go`. Live tool calls, including ended ones kept until the saved turn arrives: `live_calls.go`; the flicker detector: `flicker_helpers_test.go`. `/login` (the only providers UI): `login.go` (interaction), `login_list.go` (provider list), `login_api.go` (API-key providers, the first-run lock). |
| `internal/render/` | Rendering primitives, themes, Markdown, transcript layout and widgets. Images are delivered to the model, never drawn in the terminal. Start with `render.go`, `transcript.go`, `theme.go`. Composer-panel geometry: `panel.go`; bounded tool-output layout: `output_layout.go`. The `view` modes (full, bash only, compact) are cut in `transcript.go`; the one live row of the compact view: `compact_live.go`. |
| `internal/agent/` | Agent loop, events, retries, queues, compaction, sessions. Start with `loop.go`, `session.go`, `sessiondb.go`; history, fork and the versioned store in `history.go`, `fork.go`; the steering queue in `queue.go`, events in `events.go`. |
| `internal/llm/` | Provider API clients, streaming, model catalog, usage and protocol types. Start with `types.go`, `http.go`, `chat.go`, `responses.go`. |
| `internal/engine/` | Daemon-owned session runs, steering, resources, Bash calls, and provider reloads. Start with `engine.go`, `run.go`, `events.go`, `resources.go`. Per-session ordering: `session_commit.go` (the commit lock and `SessionVersion`), `lifecycle.go` (create/delete events). A new session lives in memory until its first message: `persist.go` (`save` gate, `adopt`, `ensureSaved`). Per-session choices of create, fork and model change (model checks, custom system prompt, spawn limits): `session_options.go`, `session_fork_options.go`. Skills named in a message (and the auto-load ones of a new session) become `skill` turns: `skill_load.go`. |
| `internal/client/` | Daemon HTTP/SSE client and authoritative session reducer (`mirror*.go`); immutable changed-field delivery in `projection.go` and `attach.go` (`Follow`), recoverable global subscriptions in `global_watch.go`. |
| `internal/cli/` | The `meno <group>` commands (including `doctor.go`; `cmd_send.go`, `cmd_sessions.go` and `cmd_tail.go` parse the session-driving commands, `backend_send.go`, `backend_sessions.go` and `backend_tail.go` talk to the daemon): parsing, daemon-backed groups (`mcp.go`, `tasks.go`, `interactive.go`, `daemon.go`) behind the `API` interface; `backend*.go` (`DaemonAPI`) implements it over the daemon socket and the `MENO_*` environment of a bash call; embedded help texts in `help/*.md` with golden files in `testdata/`. |
| `internal/form/` | Shape of the question form that `meno ask` shows to the user. |
| `internal/api/` | Shared daemon API request, response, and event types. |
| `internal/daemon/` | Daemon lifecycle, socket, HTTP resources, event streams; startup/update serialization in `transition.go`, automatic 10-second grace in `idle.go`, finite-request admission in `request_admission.go`, update admission in `update_shutdown.go`; configuration resources in `internal/configsvc/`. |
| `internal/content/` | Conversation content: images (`images.go`: prepare, store, clipboard), object tags (`objects.go`, `object_extract.go`: file, folder and skill references, `Extract`). |
| `internal/acp/` | `meno acp`: the Agent Client Protocol adapter. `rpc.go` (JSON-RPC framing), `server.go` (methods, sessions), `session.go` (daemon updates to ACP updates, prompt lifecycle, questions), `prompt.go` (prompts and slash commands), `config.go` (models, commands), `content.go` (blocks and tool cards), `mcp.go` (ephemeral daemon MCP leases), `list.go` (cursor pagination), `diagnostics.go` (opt-in metadata). Talks to the daemon only through `client`. |
| `internal/mcp/` | MCP server lifecycle, transports, JSON-RPC, tool schemas, typed results and the local result/artifact store. Start with `manager.go`, `manager_start.go`, `client.go`, `discovery.go`, `result.go`, `store.go`. Protocol eras (`Server.Protocol`, default auto: modern 2026-07-28 first, legacy fallback): `modern.go`, `modern_http.go`, `modern_headers.go`; legacy capability gating `capabilities.go`, expired-session recovery `remote_session.go`. The config service owns the shared manager lifecycle; the engine uses it and separately owns ephemeral ACP session managers in `internal/engine/mcp_scope.go`. ACP payload mapping is in `internal/mcp/acp.go`. |
| `internal/prompt/` | System prompt (identity, skills, project AGENTS.md, MCP servers, environment): `prompt.go` (inputs), `bash.go` (`Build`), `sections.go`, `xml.go`. The bash tool description: `bash_toolspec.go`. |
| `internal/config/` | Paths, settings, skills, and the JSON files that hold providers and MCP servers (`providers_file.go`, `servers_file.go`, `jsonlist.go`; one-time move out of the database: `migrate_files.go`). Single palette initialization, validation and legacy backup: `theme_file.go`. Start with `paths.go`, `settings.go`, `store.go`; custom system prompts (the `system-prompts/` directory, name rules, loading): `system_prompts.go`. |
| `internal/db/` | Shared SQLite database; versioned migrations live in `migrations.go`, opening in `db.go`, clean 0.5 epoch backup in `epoch.go`. The `providers` and `mcp_servers` tables are unused since 0.6.8 (kept for the migration). |
| `internal/process/` | Bash execution (`shell*.go`, `restricted*.go`), process tasks and supervision. Process supervisor for foreground bash calls and background tasks (`foreground.go`: launch, promote to a task, cancel): process groups, bounded rotating disk logs, durable task results; `services.go` runs the provider `run` commands. |
| `internal/update/` | `meno update` and the start-up check: latest GitHub release (the release page redirect, API as fallback), SHA-256 verified staged install (`Prepare`/`PrepareAsset` then `Prepared.Commit`), cached check in `$MENO_HOME/update.json`. |
| `internal/version/` | Single version source: `VERSION` file (embedded); `Label()` feeds the welcome screen and help title, `Number()` feeds `--version`. Bump it for every release. |
| `internal/store/` | Reusable JSON loading and atomic saving; `json.go`. |
| `internal/safe/` | Panic containment for goroutines and UI handlers: `Call`, `Recover`, `Go`; `safe.go`. |
| `internal/pathsearch/` | Fuzzy, bounded file and folder search behind the `@` picker. |
| `internal/notify/` | Built-in notification sound synthesis and platform audio playback. |
| `internal/term/` | Build-tagged pseudo-terminal end-to-end tests with a fake provider. |
| `docs/` | User documentation only (features, settings, tools, upgrading). No plans, progress logs or reports here. |
| `.report/` | Plans, specs, reports, audits and other working notes. Git-ignored; never linked from docs or code. |
| `.meno/skills/` | Project-local agent skill instructions. |
| `README.md` | User-facing setup, commands, settings, and behavior reference. |
| `Makefile` | Build, formatting, test, race, vet, and terminal-test commands. |

### Where to make common changes

- UI behavior: `internal/tui/keys.go`, `commands.go`, `modal_*.go`,
  `composer.go`; visual output: `internal/render/` and `internal/tui/draw*.go`.
- Agent behavior: `internal/agent/loop.go`, `queue.go`, `retry.go`,
  `compact.go`; application integration: `internal/tui/turn.go`.
- Tool capabilities: `internal/process/` and `internal/engine/`; update the `bash` description in `internal/prompt/bash_toolspec.go`
  when the model-facing tool contract changes.
- Storage changes: `internal/db/db.go` plus the owning configuration or
  session storage code; preserve existing data and migration behavior.
- Tests: colocated `*_test.go` files; terminal scenarios in `internal/term/`.

## Working and validation

- Read the relevant package and nearby tests before editing. Preserve unrelated
  work already present in the working tree.
- Add or update focused tests for behavior changes. Keep tests near their code.
- Format changed Go files with `gofmt`.
- Run `make test` for the Go suite; `make check` checks formatting, vet, and
  race tests. Use `make term` for real-terminal behavior changes.
- Build with `make build`. Report what was validated and any checks not run.
- UI changes must keep the flicker tests green (`internal/tui/flicker_test.go`, `make term`): no
  background that depends on state in the timeline, no screen clears after the first frame,
  and live state that is handed over to saved state must draw the same cells.
- Keep this repo map current when package responsibilities or entry points change.
- Do not create clutter in this repo. Plans, reports, audits, scratch notes and generated
  artifacts go in `.report/`, never in the root, `docs/` or `internal/`. Remove temporary
  files before you finish. Keep `docs/` to user documentation.
