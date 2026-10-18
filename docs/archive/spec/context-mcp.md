# Context, Session Retrieval, and MCP Results

Part of [the implementation specification](../plan.md). Review draft.

## Continuation summary

Use a Markdown string with sections for goal/criteria, current and superseded
decisions, completed/remaining work, constraints/exclusions, changed files and
observed Git state, validation/unverified behavior, artifact IDs, and next action/
blockers. No rigid JSON summary schema. Summaries are indexes with source links,
not replacement evidence or a second authoritative TODO/state store.

Preserve selected design frame IDs, variant names, screenshot references and agreed
animation/resize details in state/plans; include links in summary. Prompt/skills
must preserve variants unless removal is requested and freeze selected specs before
implementation. Add tested Pencil recipes as a skill, not provider-specific core.

## Agent compaction

    session.compact(summary, saveLastNMessages) -> {scheduled: true}

Validate nonempty string and nonnegative integer before scheduling. No additional
LLM request. Count messages of active context, not entire user requests. Preserve
at least N tail messages, expanding to keep replay/tool groups intact. N=0 still
retains the current compaction group and result. Keep full original history.

Allow one scheduled operation per tool batch. A duplicate throws. Apply the revision
transactionally only after all results in the current group are recorded, including
errors from code after scheduling. Report actual retained count through a trusted
event before the next model request. On storage/application failure retain old
context and report a compaction failure, without inventing success.

## Automatic safety net

Before each LLM request estimate full serialized input, schemas/images/summary and
output reserve against the known context window. Trigger at 80% by default. Keep
at least eight messages with group expansion; summary target budget 4K tokens.
All three settings are configurable. Use current model for generated summary.

Prefer actual provider usage for calibrating estimates; do not treat character/4
as precise token accounting. Unknown window requires explicit model metadata/setting;
do not guess it. Bound summary output, preserve prior context on generation failure,
and prevent loops that compact the same unchanged context without progress.
If required instructions/tail/output cannot fit, return an actionable error.
Compaction moves the active-context boundary, never deletes history or artifacts.

## Session retrieval APIs

    sessions.list({workdir?, after?, before?, limit?, cursor?})
    sessions.search(query, {workdir?, after?, before?, limit?, cursor?})
    sessions.read(id, {from?, limit?, includeToolOutput?, includeThinking?})

All local user sessions are available by default; workdir is an optional filter,
not a project/security boundary. Do not require Git project identity or block
cross-project queries. list orders by lastMessageTime descending with stable ID
tiebreak. Default page 20, maximum 100. search is local full-text title/message
search, no embeddings or model request. Escape query syntax unless deliberately
supported; parameterize storage queries and return deterministic rank/tiebreaks.

Return session/message IDs, title, workdir, lastMessageTime, provider/model,
bounded excerpts and nextCursor. Time bounds are inclusive ISO timestamps and
workdir filtering uses a normalized exact directory value, not implicit globbing.
Empty search query directs callers to list rather than an unbounded dump.

Read starts at a stable message ID (inclusive) or the beginning, with next-position
metadata. Default at most 50 records within 64 KiB. Omit thinking and tool output
by default; explicit flags permit bounded original reads. Large omitted fields
are marked with position/size, not represented as empty original text. Do not
embed attachment bytes; return real artifact references. Never expose credentials
through session metadata (message contents may contain user-supplied sensitive data).

Exclude child sessions from general list/search and user Sessions. A parent can
read its own known child ID; children cannot enumerate siblings through IDs. No
completed-child viewer. Deleting history updates the search index in the same
lifecycle transaction. Migrations index existing retained originals; cannot restore
history already discarded by older versions.

## MCP call result contract

    mcp.server.tool(args) -> {
      text, structuredContent, content, artifacts, isError,
      resultId, truncated
    }
    mcpResults.info(id) -> metadata
    mcpResults.read(id, {offset?, limit?}) -> boundedSlice

text is text, never guessed JSON. structuredContent is only explicit server data,
otherwise null. content preserves typed MCP blocks; artifacts reference actual
materialized binary/resource blocks, not paths guessed from prose. Expose mime,
size and valid local artifact references when supported. Do not auto-load or execute
an artifact merely because a tool returned it.

Transport/protocol failures throw; tool-level isError is in the result envelope.
Store complete successfully received result locally before returning a bounded
projection. Keep stable resultId and explicit omitted/truncated fields so even a
large response is inspectable without repeating external side effects. Persist
binary payloads in artifact store, not prompt base64. Limit resources defensively
and report storage/size failures explicitly rather than claiming full retention.

mcpResults.read uses an offset within a canonical UTF-8 JSON/text representation,
with a bounded byte limit and nextOffset; preserve UTF-8 boundaries and state the
returned range. info includes representation/mime/size and artifact manifest.
Default 64 KiB per read; retention and access follow the owning session history.
No arbitrary access to another parent's child result store. Existing server tools
and schema discovery remain the source of truth; update prompts/skills/examples.

## Acceptance

Test full history after repeated compaction, safe fork at old points, malformed
summary/limits, no-progress loops, all-project pagination/indexing, sensitive-field
boundaries, child access, text-versus-JSON MCP data, tool errors, binary blocks,
large-result readback, cancellation, restart, and artifact retention/deletion.
