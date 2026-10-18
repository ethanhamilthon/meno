# Diagnostics, Performance, Visual Quality, and Release

Part of [the implementation specification](../plan.md). Review draft.

## Investigation discipline

For repository-specific behavior inspect implementation before explanation. Record
observations, competing hypotheses, discriminating experiment, proven findings,
likely explanations, and remaining unknowns. Fix measured causes, not plausible
stories. Do not dismiss new cache/highlighting/performance reports using old results.

## Cache behavior

Separate latest-request and cumulative usage in metadata/UI. Define cache-read,
cache-write and input totals per provider protocol; never mix units or silently
count cache writes as hits. Unknown/inconsistent usage is explicit. Do not promise
a constant hit percentage: upstream prefix caching and routing are not controlled.

Audit deterministic message serialization, instruction/schema ordering, history,
images, metadata changes, compaction, provider/model changes and proxy behavior.
Compare controlled repeated requests through meno and direct API with stable input;
change one variable at a time. Add canonical request/prefix regression fixtures.

Default diagnostics contain request metadata, token metrics, prefix hashes/lengths,
not secrets, image bytes or full message text. Full-payload logging is explicit
opt-in debug with sensitive-data warning. Minimize/redact credentials always.
Document confirmed causes, fixes, external constraints and unresolved reproduction.

## TUI performance

Establish baseline workloads before feature development and rerun through delivery:
long transcripts, JS streaming/folding, rapid scroll/resize/input, multiple images,
concurrent agents, event bursts, large @ directory search and bounded log updates.

On a documented machine target p95 visible input/scroll response <=50 ms and typical
frame CPU work <=16.7 ms under a specified standard workload. Record workload sizes,
terminal/protocol, machine and sample method. These are workload-specific goals,
not universal speed claims. Profile CPU, allocations, lock contention and redraws.

Keep rendering free of provider calls/DB queries. Avoid recomputing all history for
each token. Use incremental layout/invalidation, viewport-aware image work, bounded
caches and async filesystem search. Avoid lost cancellation and unbounded log queues.
CI benchmarks use deterministic workloads and relative regression thresholds;
absolute wall-clock gates on variable runners must not be flaky.

## Visual regression and highlighting

Use existing deterministic TUI fixtures and add test-only screenshot comparisons,
including resize, narrow layouts, text/object selection, JS strings and forms.
No agent ui.snapshot API. Test source-to-visual mappings, not merely golden pixels.

Highlight only fully received JS tool source with modern syntax fixtures; streaming
and invalid syntax use readable fallback. Strings stay strings even if containing
other languages. Test off/lines 0/lines N, template interpolation, ellipsis color,
expand/collapse, full copy/export and unchanged execution. Assistant response
Markdown is not folded by this feature.

Test terminal graphics in actual supporting terminals separately: text-buffer
screenshots cannot prove Kitty/Sixel/iTerm correctness. Preserve reproducible
artifacts and terminal/backend-specific known limitations.

## Repository validation gates

For each phase: focused tests, gofmt changed Go files, make test, make build.
For integration/final readiness: make check (format/vet/race) and make term for
real-terminal changes, plus protocol-specific manual/integration matrix. Verify
all hand-written source/tests/docs <=300 lines, aiming <=200; extract responsibilities
rather than compressing files. Do not rewrite unrelated pre-existing modifications.

Update AGENTS.md only when entry points/responsibilities change. Keep UI in
internal/ui, interaction/wiring in internal/app, agent loop independent of rendering,
provider protocol in internal/llm, MCP in internal/mcp, JS bindings in internal/jsvm,
SQL in internal/db and configuration in internal/config. New focused process or
graphics support packages must have clear boundaries and updated repo-map entries.

## Release gate

All agreed scope ships before release, not only a reduced beta slice. Track each
requirement through implementation, tests/evidence and status. Do not claim complete
while acceptance is failed/not_checked; negotiated external limitations are explicit.

GitHub open-source release under MIT. Support macOS/Linux arm64/amd64, with source
build/go install documentation, published binaries, checksums, changelog, CI and
migration/recovery/tool-API compatibility notes. Audit license compatibility and
secret-free release artifacts. Clarify local data directories and debug privacy.

Validate clean installation, upgrade from retained fixtures, no credentials bundled,
configuration/provider setup, model discovery, shutdown, session restoration/fork,
and text-only terminal use. Review Kitty/Ghostty plus Sixel/iTerm target evidence.
Windows, auto-update, Homebrew and Docker are not release requirements. No cloud/team
features. Preserve existing custom notification commands and unrelated completed work.
