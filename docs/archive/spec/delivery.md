# Delivery Sequence and Acceptance Matrix

Part of [the implementation specification](../plan.md). Review draft.

## Working rules

All phases are one delivery scope; release is last. Continue approved implementation
without intermediate approval requests, except real blockers/material scope changes.
Keep requirement -> code -> test/evidence -> status tracking. No commits unless
requested. Respect the already modified working tree; do not claim unrelated edits.

Each phase requires focused tests, formatting, full Go tests and build. UI phases
also require terminal scenarios; final integration adds race/vet and real graphics.
Document checks not run. Do not substitute a plan item marked done for verification.

## Phase 0: baseline and persistence foundations

- Inventory current local changes, existing oversized files and responsible packages.
- Reproduce/cache diagnostic baselines; profile scrolling/input/layout; capture
  rendering/JS fixtures before changing behavior.
- Design additive schema migrations and upgrade fixtures. Introduce stable history,
  execution, request and event identities while preserving replay groups.
- Define transaction/access boundaries and artifact retention before concurrent work.
- Audit current await-on-ready-value compatibility against the synchronous host
  contract. Preserve it as the conservative final-review default, with no new
  event loop; test settled/rejected/pending results and do not hide a breaking change.

Exit: reproducible baseline, migration tests, implementation ownership map, and
compatibility evidence sufficient to make the runtime change safely.

## Phase 1: history, versions, and fork

- Separate complete source history from active context and usage projections.
- Add state/TODO storage journals and reconstruct-at-boundary repository operations.
- Implement :fork selection, safe group boundary, independence, source metadata,
  legacy warning, missing-cwd handling, and trusted rename event.
- Keep :new; remove :reset, including command references and obsolete prompt events.
- Start bounded history retrieval/indexing infrastructure over retained originals.

Exit: migration/fork/version/retention tests pass; no history deletion by compaction.

## Phase 2: reliable tool contracts

- Fresh JS runtime with local scope, explicit return/last-expression serialization,
  shared output budget, timeouts, local env and synchronous host APIs.
- Bind persistent state with immediate validation/commits and limits.
- Add session.rename/models/compact, sessions.list/search/read and context revisions.
- Add file ranges/search/inspect/hash-guarded patches and edit diagnostics.
- Introduce typed MCP results, local result/artifact store, bounded retrieval,
  prompt updates and tool-contract tests.

Exit: repeated-call/failure/state/restart, protocol replay, file conflicts, model
partial failures and MCP text/binary/large-result tests pass.

## Phase 3: unified orchestration

- One durable steering queue and serialized execution lane per session.
- Root request ownership and cancellation generations, idle wake, safe injection,
  event provenance, acknowledgment and wait-notification deduplication.
- Define old-event/current-request ownership and interrupted recovery exactly.
- Wire :stop/Ctrl+C, session switch, :new and normal shutdown consistently.

Exit: race tests cover events during stream/tool/form/idle/stop and no double loops.

## Phase 4: forms, plans, processes, and delegation

- Multi-question askUser API/UI, navigation, Other/multi semantics, Submit/Cancel,
  timeout, no partial leakage, interrupt/exit behavior.
- TODO/criteria APIs, visible collapsible read-only UI, archives, evidence and
  honest partial completion; hook versions into fork.
- Process supervisor, bounded disk logs, tasks API and :tasks modal, notify and
  five-second graceful/forced cleanup.
- Subagent lifecycle/context/model/cwd, reports/block/resume, result envelopes,
  direct-user steering, :agents modal, usage accounting and prohibited bindings.
- Update prompt/skills to use new APIs and continue agreed plans to completion.

Exit: end-to-end delegation/process/stop/notification scenarios and cost accounting
pass; completed children are internal-only and no process auto-restarts on startup.

## Phase 5: atomic input and JS display

- Typed message segments, persistence/legacy recognition and canonical serialization.
- Explicit skill selection, fixed paths, unfiltered asynchronous @ fuzzy completion,
  directory navigation, symlink policy, code-safe clipboard restoration.
- Atom-aware operations for all modes, mouse/selection, wrapping, copy/undo/queue,
  restored user-message highlighting and fork.
- Completed-JS highlighting and string-only off/lines folding without source changes.

Exit: edit property tests, narrow/resize goldens and terminal input scenarios pass.

## Phase 6: terminal graphics

- Viewport image placements/caching and coordinated terminal transport.
- Kitty/Sixel/iTerm backends, capability routing, manual setting, safe fallbacks.
- Attachments/view() history rendering, scrolling/clipping/modal/session cleanup.

Exit: deterministic backend tests and documented real-terminal protocol matrix,
including Kitty/Ghostty and unsupported/tmux fallback, meet image acceptance.

## Phase 7: regression closure and performance

- Rerun cache comparisons/profile workloads against phase-0 baselines.
- Fix confirmed defects; add targeted cache/JS/performance regressions.
- Add test-only screenshots/comparisons and tested Pencil/design-provenance skill.
- Audit all user requirements, API/error docs, privacy and migration/recovery cases.
- Run make test, make build, make check, make term, and real image tests.

Exit: all requirements have evidence or explicitly approved limitations; no silent
partial implementation, data loss, shared-state race or critical rendering blocker.

## Phase 8: first release

Prepare MIT/GitHub docs, macOS/Linux four architecture binaries, checksums,
changelog, source/go install and upgrade instructions, CI and clean-install checks.
Release only after phase-7 gates, with no hidden reduction of TODO/tasks/agent scope.

## Requirement-to-validation matrix

| Requirement | Minimum evidence |
| --- | --- |
| Fresh JS/state | repeat declarations; strict JSON; DB failure; restart; limits |
| File tools | ranges/Unicode; bounded search; stale hash; ambiguity/no writes |
| MCP | text preserved; structured data; isError; binary; bounded readback |
| session metadata | <=6 words; fork rename; models partial/stale/no secrets |
| Complete history | repeated compaction preserves originals and stable IDs |
| Fork | historical versions; no future summary; shared-artifact deletion safety |
| Search/list/read | cross-workdir pages; stable positions; child exclusion |
| Steering/stop | event A during B; prompt/form/tool boundaries; cancellation race |
| askUser | all types/Other; navigation/resize; Cancel exception; no partials |
| TODO/criteria | archives/versions; independent child plans; honest status |
| Tasks | group cleanup; lifetime vs wait; rotated logs; notify dedup; exit |
| Subagents | explicit context/model/cwd; no child spawn/task/ask; block/resume |
| Agent UI | only working children; user-source tags; no child Sessions/history UI |
| @ picker | all files including .git; fuzzy/paths/spaces; symlink/no recursion |
| Atomic objects | motion/edit property tests; clipboard; wraps/legacy/fork |
| JS display | completed-source syntax; string folding; full copy/execution |
| Images | three protocols; view/attachments; scroll/resize/modal/fallback |
| Cache | request/cumulative units; canonical prefixes; controlled comparison |
| Performance | baseline + profiles; p95/frame targets; relative CI regressions |
| Design provenance | stable IDs/selection; variants preserved; tested recipes |
| Release | upgrade/clean install; artifacts/checksums/license/CI/platform matrix |
