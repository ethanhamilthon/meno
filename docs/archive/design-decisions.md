# Design Interview: Decision Index

Status: interview consolidated into a review specification; awaiting final approval.
Date: 2026-10-17.

The working specification and phased acceptance plan are in [docs/plan](plan.md).
That document and its linked spec files supersede intermediate interview proposals.
No implementation has been started by this design session.

## Confirmed scope

- All backlog ideas and agreed additions are implemented before the first release.
- GitHub open source, MIT, macOS/Linux arm64/amd64; no Windows commitment.
- Full autonomy within task; preserve user data/history through migrations.
- Existing TUI/Vim foundation, no dedicated Git/worktree functionality.
- Fresh JS runtime per call and explicit durable state map with version history.
- State set validates strict JSON and commits immediately, even if later JS fails.
- Deep-copy reads; get/has/delete/listKeys/listValues/listEntries; no clear.
- Per-value/session defaults 1/10 MiB; state is not automatically added to prompt.
- session.rename (<=6 words, ideally 4), models, and scheduled compact.
- Full original history independent of active context; global bounded retrieval.
- :new retained, :reset removed, :fork clones state/TODO at a chosen safe boundary.
- One steering system and serialized lane for user/child/task events.
- :stop cancels the active request's owned work, not older requests' background jobs.
- Main agent alone launches tasks/subagents; no nested agents or child askUser/tasks.
- No subagent concurrency limit, read-only/full modes, or obligatory cost budget.
- Explicit child context/cwd/model, two-way reports/block/resume, repeatable results.
- :agents is current-session working children only, with live direct-user steering.
- No child Sessions entries or completed-child user viewer; histories persist internally.
- Noninteractive tasks with disk logs, notifications, wait/cancel, :tasks UI and cleanup.
- Multi-question askUser with all-required/custom answers, multi-select/navigation,
  Submit, Cancel exception, and no Esc cancellation or unfinished-form restoration.
- Agent-managed visible/collapsible TODO, separate criteria/evidence and archives.
- Atomic image/skill/file/folder objects throughout editors and old user messages.
- Explicit autocomplete selection; fixed path links, no file tracking/content injection.
- @ search ignores no files/directories, including .git; symlinks don't auto-recurse.
- Internal object clipboard; strict full tags for external links/skills; literal code examples.
- Kitty/Sixel/iTerm inline timeline previews, no image viewer or composer thumbnails.
- Completed-JS highlighting; string-content off/lines folding, not assistant-text hiding.
- File ranges/search/inspect/hash-guarded patches and strict edit diagnostics.
- Typed MCP envelopes and bounded local result/artifact readback, no guessed JSON.
- Evidence-based cache fixes, measured TUI goals, test-only visual snapshots.
- Provenance through state/plans/skills, no separate design artifact manager.

## Interview reference notes

- [Input objects](design-input-objects.md)
- [Inline images](design-inline-images.md)
- [Context](design-context.md)
- [Fork](design-fork.md)
- [Continuous final round and explicit supersessions](design-interview-final-round.md)

These notes preserve how decisions were reached. Any older open-question text in
reference notes is historical; the consolidated specification records final behavior.
Async syntax did not receive an interview answer. The final-review conservative
default preserves existing await-on-ready-value compatibility and synchronous host
tools without a new event loop; validate it before rewriting runtime execution.

## Corrections that must not be lost

- :reset currently only restarts the VM, not creates a session. Remove it; use :new.
- No worktree manager, read-only subagent mode, completed-child viewer, or ui.snapshot.
- No .gitignore filtering in @. File content is never automatically embedded.
- String folding concerns JS literals, not assistant response blocks/Markdown.
- New actions after old-event steering belong to the active request, not the event source.
- Fork reconstructs historical state/TODO, not today's versions at an old boundary.

## Interview workflow

Use multiple separate askUser calls per assistant turn for independent frontier
questions; do not combine a whole round into one tool question. Inspect repository
facts rather than asking the user to supply facts available in code. Confirm shared
understanding before implementation. Preserve unrelated working-tree changes.
