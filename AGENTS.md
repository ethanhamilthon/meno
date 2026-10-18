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
  `internal/mcp`, and JavaScript tool bindings in `internal/jsvm`.
- Keep SQL/schema concerns in `internal/db`, configuration persistence in
  `internal/config`, session persistence in the agent's session storage files,
  and reusable JSON file persistence in `internal/store`.
- Extend the existing responsible package before creating a parallel abstraction.

## Repo map

meno is a Go terminal coding agent with a tcell UI, a persistent goja JavaScript
VM, provider APIs, MCP integrations, and SQLite persistence.

| Path | Responsibility / starting points |
| --- | --- |
| `cmd/meno/main.go` | CLI entry point; terminal setup and application lifecycle. |
| `internal/app/` | TUI controller, state, input, commands, modals, session/runtime wiring. Start with `app.go`, `state.go`, `runtime.go`, `turn.go`. |
| `internal/ui/` | Rendering primitives, themes, Markdown, transcript layout, widgets. Start with `render.go`, `transcript.go`, `theme.go`. |
| `internal/agent/` | Agent loop, events, retries, queues, compaction, sessions. Start with `loop.go`, `session.go`, `sessiondb.go`; history/fork/versioned store in `history.go`, `fork.go`, `kv.go`; steering queue in `events.go`; subagent records in `agents_store.go`; retrieval in `retrieval.go`. |
| `internal/llm/` | Provider API clients, streaming, model catalog, usage and protocol types. Start with `types.go`, `http.go`, `chat.go`, `responses.go`. |
| `internal/jsvm/` | Fresh-per-call JS runtime and its bindings: filesystem (`fs_*.go`), shell, HTTP, session, tasks, agents, askUser forms, MCP, and `help` (topic texts in `help/*.md`). Start with `vm.go`, `exec.go`, `compile.go` and `prelude.js`. |
| `internal/mcp/` | MCP server lifecycle, transports, JSON-RPC, tool schemas, typed results and the local result/artifact store. Start with `manager.go`, `client.go`, `result.go`, `store.go`. |
| `internal/prompt/` | System prompt assembly and built-in tool instructions; `prompt.go`. |
| `internal/config/` | Paths, settings, providers, skills, MCP configuration, and the read-only `provider.json`/`mcp.json` overlays (`overlay.go`). Start with `paths.go`, `settings.go`, `store.go`. |
| `internal/db/` | Shared SQLite database; additive versioned migrations live in `migrations.go`, opening in `db.go`. |
| `internal/proc/` | Background process supervisor: process groups, bounded rotating disk logs, durable task results; `daemon.go` runs the provider `run` commands. |
| `internal/update/` | `meno update` and the start-up check: latest GitHub release, SHA-256 verified install over the executable, cached check in `$MENO_HOME/update.json`. |
| `internal/version/` | Single version source: `VERSION` file (embedded); `Label()` feeds the welcome screen and help title, `Number()` feeds `--version`. Bump it for every release. |
| `internal/store/` | Reusable JSON loading and atomic saving; `json.go`. |
| `internal/safe/` | Panic containment for goroutines and UI handlers: `Call`, `Recover`, `Go`; `safe.go`. |
| `internal/attach/` | Attachment parsing and source loading. |
| `internal/objtag/` | Text form of input objects (file, folder, skill references): tags, parsing and escaping. |
| `internal/pathsearch/` | Fuzzy, bounded file and folder search behind the `@` picker. |
| `internal/termimg/` | Image encoding for the Kitty, Sixel and iTerm terminal graphics protocols. |
| `internal/notify/` | Built-in notification sound synthesis and platform audio playback. |
| `internal/term/` | Build-tagged pseudo-terminal end-to-end tests with a fake provider. |
| `.meno/skills/` | Project-local agent skill instructions. |
| `README.md` | User-facing setup, commands, settings, and behavior reference. |
| `Makefile` | Build, formatting, test, race, vet, and terminal-test commands. |

### Where to make common changes

- UI behavior: `internal/app/keys.go`, `commands.go`, `modal_*.go`,
  `composer.go`; visual output: `internal/ui/` and `internal/app/draw*.go`.
- Agent behavior: `internal/agent/loop.go`, `queue.go`, `retry.go`,
  `compact.go`; application integration: `internal/app/turn.go`.
- Tool capabilities: `internal/jsvm/`; update `internal/prompt/prompt.go`
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
