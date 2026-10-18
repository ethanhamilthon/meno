# Storage, History, and Session Forking

Part of [the implementation specification](../plan.md). Review draft.

## Ownership and durable records

Keep SQL/schema ownership in internal/db, session persistence in internal/agent,
and settings persistence in internal/config. Tool bindings do not issue ad hoc SQL.
Extend responsible packages before introducing a parallel persistence framework.

Persist independently:

- Session identity, kind (main/child), parent, cwd, title, provider/model/effort.
- Stable message/event IDs and a monotonic session sequence, not timestamp-only order.
- Root request ID, execution/step IDs, source kind, and child/task ownership.
- Full message/replay/tool-result history and immutable attachment references.
- Active-context selection and summary revisions, separate from original history.
- State entries and a version journal with value/tombstone and sequence boundary.
- TODO plans/items/criteria, revision history, archives, and evidence references.
- Questionnaires, submitted answers, task/child lifecycle, usage, and artifacts.
- Fork source session/boundary and inherited-history attribution.

Model replay groups remain indivisible. Append annotations/revisions where mutable
projections such as run statistics need updating; do not destroy original evidence.
Store large binary artifacts on disk with metadata/ownership in the DB. Session
and artifact references must not expose credentials through model-discovery APIs.

## Commit and version semantics

A successful state set/delete commits before returning. Write the current value
and its journal record in the same transaction. TODO mutations follow the same
rule. Associate mutations with the executing durable step and message boundary.
Each session has its own versions; child sessions never share mutable state.

Use atomic transactions for context revision and fork creation. A failed operation
leaves the old projection usable. Fork snapshots include every committed mutation
through the selected completed boundary, including mutations before a failed call.

## Session lifecycle

:new creates an independent empty session with a new ID in the current directory,
using the normal model defaults. Do not destroy or transfer old session work.
Remove :reset from commands, autocomplete, help, tests, and documentation. Internal
VM recovery may recreate an interpreter but must not erase state or repeat effects.

Explicit session deletion first cancels and joins its managed work, then removes
its owned messages, child histories, state, plans, logs, and result references.
Garbage-collect immutable bytes only after no owning session references remain.
Deletion of one fork cannot break images/history referenced by another fork.

## :fork workflow

1. Select a user/assistant message; default to the latest completed message.
2. Include that message; expand the boundary to a complete replay group if needed.
   An unfinished streamed message is not a selectable completed boundary.
3. Transactionally create a new main session and copy eligible history, model,
   effort, cwd, state, TODO, criteria, and applicable summary.
4. Store source session/boundary. Never reuse a summary containing later facts;
   reconstruct from original messages when the chosen boundary predates it.
5. Open the fork without a model call. The next explicit user message/Continue
   delivers a trusted fork event requiring session.rename again.

The fork is independent after creation. Immutable artifacts may be shared, but
mutable data is not. Do not clone live tasks/children, transfer request ownership,
rewind Git/files, or charge inherited historical usage as new fork spending.
Missing cwd requires explicit directory selection rather than a silent fallback.

For legacy boundaries without version history, warn and offer conversation-only
fork with empty state/TODO. Never pretend the current values existed in the past.

## Migration and recovery

Use additive versioned migrations with fixture tests for existing/legacy databases,
including user history, attachments, settings, and already uncommitted feature work.
Import existing records once with stable IDs and preserve replay ordering. Existing
summaries cannot reconstruct history already discarded by old versions: retain
available data and mark the coverage boundary honestly, without fabricated records.

Crash durability means committed transactions under the existing SQLite durability
settings, not a promise against every power/storage failure. On restart mark
unfinished execution interrupted; never retry commands or LLM requests silently.
Fork journal coverage begins at migration; earlier state is explicitly unavailable.

## Acceptance

- Upgrade fixtures twice without loss/duplication; failures leave usable data.
- Fork before/after successful and failed tool calls reconstructs correct state.
- A fork before compaction has no future summary facts; protocol groups stay whole.
- Fork writes and deletions do not mutate the source or break shared artifacts.
- :new preserves old sessions; :reset is absent; deletion cancels only owned work.
