# Release Blockers and First-Release Sequencing

Follow-up to [the session review and improvement backlog](archive/agent-improvement-review.md).

User-reported blockers to investigate and resolve before release preparation:
- Cache behavior: the supplied screenshot shows 85% cache hit, 359.1K input,
  8.2K output, and 49.8K/922K context. Investigate reproducible cache losses;
  this percentage alone does not establish a cause. Compare per-request and
  cumulative usage, stable request prefixes, history/images, compaction,
  model changes, and proxy/upstream behavior. Fix confirmed causes and add
  regression coverage; do not dismiss this report based on earlier findings.
- Code highlighting: reproduce the reported rendering problems, identify
  affected languages/themes and streaming versus completed code blocks,
  fix the causes, and add focused rendering regression tests.
- TUI performance: reproduce and profile slow scrolling, rendering, flicker,
  and input latency, including long transcripts and streaming. Fix measured
  bottlenecks and add performance/regression checks before release preparation.

Additional input and attachment requirements:
- Path autocomplete and file picker triggered by @ in the composer. Support
  finding and selecting file paths and inserting the selected file reference.
- Treat images, skills, and file references as indivisible objects throughout
  input and selection, rather than editable fragments of their display text.
  Apply this consistently to cursor movement, mouse and keyboard selection,
  insertion, deletion, replacement, cut/copy/paste, and normal/insert/visual
  editing modes. No operation should leave a partial object token or detach
  its visible representation from the referenced item.
- Add focused regression tests for the file picker and atomic object behavior,
  including selections spanning ordinary text and multiple object types.

After the improvement/fix work above, begin preparation for the first release.
The subsequent [implementation specification](archive/plan.md) includes TODO, tasks,
subagents, and inline terminal images before release. It supersedes the earlier
idea-capture-only scope of the original review.

## Status (2026-10-18)

| Blocker | Status | Evidence / what is left |
| --- | --- | --- |
| Cache behavior | Instrumented, cause not confirmed | Usage units, request fingerprints and the opt-in payload log (`MENO_DEBUG_PAYLOADS=1`) exist (llm/fingerprint_test.go). The 85% report has not been reproduced against a real provider. |
| Code highlighting | Fixed for completed code | ui/jscode_test.go and the split markdown tests cover it. Streaming blocks are shown plain on purpose. |
| TUI performance | Partly measured | Benchmarks and the per-turn layout cache are in docs/performance.md. The plan block no longer moves while a reply streams (app/plan_anchor_test.go). No p95 on a real terminal has been measured. |
| `@` picker, atomic input objects | Implemented | internal/pathsearch, internal/objtag and their tests. |
