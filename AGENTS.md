# Repository instructions

## File size

- Keep files small and focused: aim for 200 lines or fewer, with a hard maximum
  of 300 lines for hand-written source, tests, and documentation.
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
- Keep rendering in `internal/ui` and application interaction/wiring in
  `internal/app`; do not put provider HTTP calls or database queries in views.
- Keep the agent loop independent of terminal rendering. Pass events and data
  through explicit interfaces instead of reaching into UI state.
- Keep provider API/protocol code in `internal/llm`, MCP transport code in
  `internal/mcp`, and Bash tool execution in `internal/tools`.
- Keep SQL/schema concerns in `internal/db`, configuration persistence in
  `internal/config`, session persistence in the agent's session storage files,
  and reusable JSON file persistence in `internal/store`.
- Extend the existing responsible package before creating a parallel abstraction.

## Repo map

meno is a Go terminal coding agent with a tcell UI, a daemon-owned engine,
Bash tools, provider APIs, MCP integrations, and SQLite persistence.

| Path | Responsibility / starting points |
| --- | --- |
| `cmd/meno/main.go` | CLI entry point; terminal setup and application lifecycle. |
| `internal/app/` | TUI controller, state, input, commands, modals, session/runtime wiring. Start with `app.go`, `state.go`, `runtime.go`, `turn.go`. Text fields outside the composer: `textedit*.go` (`textEdit`), forms in `forms.go`. `:login` (the only providers UI): `login.go` (list), `login_flow.go` (OAuth login modal), `login_oauth.go` (helper glue, the derived `oauth` provider), `login_api.go` (API-key providers, the first-run lock). |
| `internal/ui/` | Rendering primitives, themes, Markdown, transcript layout, widgets. Start with `render.go`, `transcript.go`, `theme.go`. |
| `internal/agent/` | Agent loop, events, retries, queues, compaction, sessions. Start with `loop.go`, `session.go`, `sessiondb.go`; history/fork/versioned store in `history.go`, `fork.go`, `kv.go`; steering queue in `events.go`; retrieval in `retrieval_read.go`. |
| `internal/llm/` | Provider API clients, streaming, native web search, model catalog, usage and protocol types. Start with `types.go`, `http.go`, `chat.go`, `responses.go`, `websearch.go`. |
| `internal/engine/` | Daemon-owned session runs, steering, resources, Bash calls, and provider reloads. Start with `engine.go`, `run.go`, `events.go`, `resources.go`. |
| `internal/tools/` | Bash execution, process tasks, filesystem commands, and restricted command parsing. |
| `internal/client/` | Daemon HTTP/SSE client and session mirror. |
| `internal/cli/` | The `meno <group>` commands: parsing, local file commands (`files.go`, `file_*.go`), daemon-backed groups (`mcp.go`, `tasks.go`, `interactive.go`, `daemon.go`) behind the `API` interface; `daemonapi/` implements it over the daemon socket and the `MENO_*` environment of a bash call. |
| `internal/clihelp/` | Embedded help texts of the commands (`help/*.md`, golden files in `testdata/`). |
| `internal/form/` | Shape of the question form that `meno ask` shows to the user. |
| `internal/routing/` | Per-request effort routing (`model_router.go`, `routing.go`). |
| `internal/proto/` | Shared daemon API request, response, and event types. |
| `internal/daemon/` | Daemon lifecycle, socket, HTTP resources, event streams; configuration resources in `cfgsvc/`. |
| `internal/mcp/` | MCP server lifecycle, transports, JSON-RPC, tool schemas, typed results and the local result/artifact store. Start with `manager.go`, `client.go`, `result.go`, `store.go`. |
| `internal/websearch/` | Brave, Tavily and keyless DuckDuckGo search backends; `search.go` dispatches by backend. |
| `internal/prompt/` | System prompt assembly: `prompt.go` (`Build`, inputs), `sections.go` (one XML element per section), `xml.go` (`element`, `escapeBody`), `bash.go` (the Bash prompt and its guidelines) and the Bash tool description (`bash_toolspec.go`). |
| `internal/config/` | Paths, settings, providers, skills, snippets, MCP configuration, and the read-only `provider.json`/`mcp.json` overlays (`overlay.go`). Start with `paths.go`, `settings.go`, `store.go`, `snippets.go`. |
| `internal/db/` | Shared SQLite database; additive versioned migrations live in `migrations.go`, opening in `db.go`. |
| `internal/proc/` | Background process supervisor: process groups, bounded rotating disk logs, durable task results; `daemon.go` runs the provider `run` commands. |
| `internal/update/` | `meno update` and the start-up check: latest GitHub release (the release page redirect, API as fallback), SHA-256 verified install (`Asset`/`InstallAsset`, also used for cliproxyapi), cached check in `$MENO_HOME/update.json`. |
| `internal/modes/` | Built-in modes (`#brainstorm`, `#goal`), their texts and composition (`compose.go`). Goal continuation is in `agent/goal.go`; session mode controls are in `app/modes.go`. |
| `internal/cliproxy/` | The cliproxyapi helper behind OAuth logins: install and update from its GitHub releases (`install.go`), config and command (`config.go`), state/keys (`manager.go`), management API client for logins and accounts (`api.go`), the six services (`services.go`). The process itself runs under `proc.Daemons`. |
| `internal/version/` | Single version source: `VERSION` file (embedded); `Label()` feeds the welcome screen and help title, `Number()` feeds `--version`. Bump it for every release. |
| `internal/store/` | Reusable JSON loading and atomic saving; `json.go`. |
| `internal/safe/` | Panic containment for goroutines and UI handlers: `Call`, `Recover`, `Go`; `safe.go`. |
| `internal/attach/` | Attachment parsing and source loading. |
| `internal/objtag/` | Text form of input objects (file, folder, skill references): tags, parsing, escaping and `Extract` (take some objects out, keep the rest). |
| `internal/pathsearch/` | Fuzzy, bounded file and folder search behind the `@` picker. |
| `internal/termimg/` | Image encoding for the Kitty, Sixel and iTerm terminal graphics protocols. |
| `internal/notify/` | Built-in notification sound synthesis and platform audio playback. |
| `internal/term/` | Build-tagged pseudo-terminal end-to-end tests with a fake provider. |
| `docs/` | User documentation only (features, settings, tools, upgrading). No plans, progress logs or reports here. |
| `.report/` | Plans, specs, reports, audits and other working notes. Git-ignored; never linked from docs or code. |
| `.meno/skills/` | Project-local agent skill instructions. |
| `README.md` | User-facing setup, commands, settings, and behavior reference. |
| `Makefile` | Build, formatting, test, race, vet, and terminal-test commands. |

### Where to make common changes

- UI behavior: `internal/app/keys.go`, `commands.go`, `modal_*.go`,
  `composer.go`; visual output: `internal/ui/` and `internal/app/draw*.go`.
- Agent behavior: `internal/agent/loop.go`, `queue.go`, `retry.go`,
  `compact.go`; application integration: `internal/app/turn.go`.
- Tool capabilities: `internal/tools/` and `internal/engine/`; update `internal/prompt/prompt.go`
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
- Keep this repo map current when package responsibilities or entry points change.
- Do not create clutter in this repo. Plans, reports, audits, scratch notes and generated
  artifacts go in `.report/`, never in the root, `docs/` or `internal/`. Remove temporary
  files before you finish. Keep `docs/` to user documentation.
