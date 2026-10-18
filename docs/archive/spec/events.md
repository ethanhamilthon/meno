# Unified Steering, Request Ownership, and Cancellation

Part of [the implementation specification](../plan.md). Review draft.

## Identities

A session contains conversation/history and one serialized execution lane. A root
request is the unit stopped by :stop. A request may have several continuations and
own many child agents and background processes. A tool invocation is not a request.

A new user message after prior work completes starts a new request. User steering
while work is active belongs to that active request. Child-directed user messages
do not create another request or transfer the child's original ownership.

Keep source provenance separate from current execution ownership. Never infer a
user message or system event from payload tags alone. Trusted metadata originates
in application wiring, not tool-provided text.

## Single steering pipeline

Use the same durable queue for user input, parent/child messages, child completion,
and notify-enabled task completion. Each item has a stable event ID, source, owner,
time, delivery/ack state, and bounded payload/artifact references.

- During execution: inject queued items before the next model request.
- During a tool call: do not interrupt or split call/result replay groups.
- During askUser: queue until the form resolves; do not close or overwrite it.
- When idle: an eligible event schedules a continuation on the same session lane.
- Coalesce ready events without losing ordering/provenance; persist before waking.
- Never run two main loops for a session or call back into its executing JS VM.

Events are model context, not source-code instructions. Render their provenance
clearly and use explicit trusted wrappers. Keep prompt/system schemas stable when
possible; do not rebuild unrelated context on every progress notification.

## Ownership of subsequent actions

If old task A completes while request B is active, its event enters B as steering.
A retains its original owner; newly spawned work in B belongs to B, even if motivated
by the event. Do not silently switch execution ownership to A. This supersedes the
earlier proposal that all event-motivated actions inherit the old request ID.

If idle, an automatic continuation uses the source event's request ID. A cancelled
request cannot auto-resume. When idle events from different requests are ready,
schedule them serially, retaining their separate owners. Expose active ownership
in status/diagnostic metadata; cancellation reads the actual active request ID.

## Completion and deduplication

Child completion with remaining work makes the parent waiting, not fully finished;
queued completion steering wakes it to inspect results. Progress alone must not
produce recursive wake loops without new information.

A wait and completion notification observe the same durable terminal result. An
explicit successful wait acknowledges that completion for model notification;
result/status reads remain repeatable. A timed-out wait does not acknowledge it.
Use event IDs and execution generations to prevent replay/restart/double-delivery
races. Do not promise exactly-once external effects; never retry whole commands
just because a notification was redelivered.

## :stop and individual cancellation

:stop cancels the active root request and its owned running child agents/tasks.
Tasks from prior requests remain untouched. Prevent new owned launches after the
cancellation marker; cancel races deterministically. Suppress pending auto-wake
for that request, retain results/history, and mark cancellations explicitly.

agents.cancel(id) targets one child and its active synchronous tools, not siblings.
tasks.cancel(id) targets one process group. Cancelling a child must not cancel
unrelated main-agent tasks. Existing Ctrl+C mappings should use the same request
cancellation service where they stop execution.

Effects already committed to files, services, or state are not rolled back. Shell
children escaping a managed process group are outside guaranteed cancellation.

## Exit, crash, and session switching

A normal application exit cancels all sessions' managed work without an additional
approval prompt, joins workers, performs graceful/forced process cleanup, and
persists final states. Do not leave a blocked form preventing shutdown.

Switching sessions/:new does not cancel old work. :cd/model changes cannot mutate
a running call's captured configuration; directory changes that invalidate active
work are rejected until it is stopped, following current command safety rules.

After crash/restart, mark unfinished jobs/questions/requests interrupted; never
automatically restart commands or LLM requests. Best-effort cleanup checks process
identity/start-time ownership; a stale PID alone is never sufficient to kill.
Document that SIGKILL cannot guarantee prior process cleanup.

## Acceptance

Exercise child/task completion during stream, tool execution, askUser, idle, stop,
and session switch; simultaneous events; wait-notify races; DB failure; and restart.
Test A-server/B-task stop ownership and event-A-during-B new-work ownership exactly.
