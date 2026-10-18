# Changelog

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
