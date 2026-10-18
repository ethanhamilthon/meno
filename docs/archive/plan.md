# Meno: Complete Implementation Specification and Delivery Plan

Status: review draft, awaiting final user confirmation. No implementation approval
is implied by this document. Interview decisions override the original backlog.
Date: 2026-10-17.

## Product contract

Deliver all agreed improvements before the first GitHub release under MIT.
Target individual developers using the local Go TUI on macOS/Linux arm64/amd64.
Keep the existing TUI and Vim foundations. Preserve user data and history while
allowing pre-release tool API changes. No worktree manager: use sh and Git.

Agents operate autonomously without additional approval prompts. Delegation does
not bypass OS/service permissions. The main agent remains responsible for scope,
verification, integration, and accurate completion reporting. Partial work must
not be presented as fully verified. No commits unless separately requested.

## Specification map

| Document | Responsibility |
| --- | --- |
| [Storage](spec/storage.md) | Records, versions, migrations, retention, fork |
| [Runtime/tools](spec/runtime-tools.md) | JS, state, session metadata, file tools |
| [Events](spec/events.md) | Steering, request ownership, cancellation, recovery |
| [Questions/TODO](spec/questions-todo.md) | askUser forms, checklists, criteria |
| [Tasks/agents](spec/tasks-agents.md) | Process and delegation APIs, lifecycle, UI |
| [Input/rendering](spec/input-rendering.md) | Objects, @ picker, skills, JS strings |
| [Images](spec/images.md) | Kitty/Sixel/iTerm timeline rendering |
| [Context/MCP](spec/context-mcp.md) | Compaction, session retrieval, MCP artifacts |
| [Quality/release](spec/quality-release.md) | Cache, profiling, validation, release |
| [Delivery](spec/delivery.md) | Dependencies, phases, completion matrix |

The specification distinguishes confirmed product behavior from engineering
choices. Exact internal type/table names may change without altering contracts.
The conservative async default for final review is to keep synchronous host tools
and existing await-on-ready-value compatibility, with no new event loop. Audit
compatibility before the runtime rewrite; pending async work must not escape a call.

## Principal decisions

- Fresh JS runtime per invocation; state is an explicit DB-backed JSON map.
- Full original history survives compaction. Version state/TODO for historical fork.
- :new creates an empty session; remove :reset; :fork clones at a safe message point.
- session.rename names only the current user-facing session, <=6 words, ideally 4.
- session.models exposes enabled providers' models and known capabilities safely.
- One steering queue for user input, child messages/completion, and task completion.
- :stop cancels the active root request and its owned work, not every session task.
- Tasks are noninteractive processes, survive turns only while meno is open,
  and support logs/wait/cancel/notify. Only the main agent launches them.
- Only the main agent launches subagents, without concurrency limits or mode flags.
  Explicit task/context, optional cwd/model; no child askUser/tasks/nested spawning.
- :agents shows only working children of the current session, with direct messages.
  No user-facing completed-child viewer or child entries in Sessions.
- TODO is agent-maintained, persisted, visible/collapsible, not user-editable.
  Acceptance criteria are independent from work steps.
- askUser accepts multiple required questions, multi-select, custom text, navigation,
  and explicit Submit/Cancel. Cancel throws; Esc does not close.
- File/folder references are fixed paths only; never auto-read or track them.
- @ search excludes nothing, including .git and ignored files. Explicit selection
  creates an atomic object; plain typed paths and skill names remain text.
- Objects are atomic across editing, selection, copying, old messages, and wrapping.
- Timeline displays images from attachments and view() with graphics fallbacks.
- Highlight completed JS source, not streaming fragments or languages inside strings.
  Fold JS literal content using configurable off/lines N, not assistant-response text.
- Session retrieval searches all local user sessions with bounded pagination.
- MCP results are typed envelopes with bounded local artifact retrieval.
- Cache fixes require evidence; performance and visual regressions require measurement.

## Explicit exclusions

No dedicated Git/worktree UI/API; no user-editable TODO; no subagent modes/allowlist;
no nested delegation, child askUser, child background tasks, child Sessions entries,
or completed-child viewer; no PTY/stdin for tasks; no image viewer/zoom/pan; no agent
ui.snapshot API; no cloud sync, team accounts, Windows commitment, auto-update,
Docker, or Homebrew requirement for the first release.

## Approval and execution boundary

Review this index and the linked contracts together. Implementation begins only
after confirmation of shared understanding. Preserve unrelated working-tree edits.
Run focused tests during each phase and the full required gates before release.
Where an upstream capability cannot be controlled, record evidence and limitations;
do not silently replace agreed behavior with a weaker implementation.
