# Background Tasks and Subagents

Part of [the implementation specification](../plan.md). Review draft.

## Shared lifecycle rules

Use stable IDs, owning session/root request, creation/start/end times, explicit
status transitions, and durable final results. Unknown/unauthorized IDs throw.
Cancellation is idempotent; a completed resource is not restarted by cancel/wait.
Wait timeout never cancels the underlying resource. No mandatory cost/step budget.

## Process tasks API

    tasks.start(command, {cwd?, env?, name?, timeout?, notify?}) -> {id}
    tasks.list({includeFinished?}) -> task[]
    tasks.status(id) -> task
    tasks.logs(id, {cursor?, limit?, stream?}) -> logSlice
    tasks.wait(id, {timeout?}) -> taskResult
    tasks.cancel(id) -> task

Available to main agents only. start takes a shell string, launches /bin/sh -c,
with inherited call cwd/environment plus explicit overrides. Validate cwd, env,
finite nonnegative timeout, and notify before spawn. Pin captured configuration.
Stdin is closed; no PTY, terminal emulation, or interactive input API.

start returns after successful process creation, not after exit. Startup failure
throws; a later nonzero exit is failed task data. Default lifetime has no limit.
JS timeout only limits startup; start.timeout limits process lifetime in seconds.

Task statuses: starting, running, succeeded, failed, cancelled, timed_out,
interrupted. Return nullable exitCode and reason where no normal exit occurred.
wait returns status/exitCode/reason and waitTimedOut when still running. Successful
wait acknowledges completion steering; timed-out wait does not. Logs stay separate.

Manage a process group. Cancel/shutdown sends graceful termination, then forces
termination after five seconds if needed. Capture final output and reap processes.
Do not promise cleanup for processes deliberately escaping the group.

## Logs and notifications

Bound disk logs to 10 MiB per task by default, configurable, retaining newest data.
Keep stdout/stderr identity, sequence/byte offsets and timestamps; cross-stream
ordering is capture order, not a promise about the process's exact write order.
Return explicit lost ranges/truncated flags. Cursor identifies a stable position;
if it expired due to rotation, return the first available position and gap.

Default logs read: last 200 lines within 64 KiB. stream selects stdout/stderr/all;
limit bounds records/output bytes, never unbounded prompt output. Preserve logs
through normal completion/restart until owning session-history deletion.

notify defaults true. Deliver ID/status/exitCode/log reference via unified steering,
not the full log. notify:false suppresses automatic agent wake, not UI updates or
explicit status/wait. Deduplicate wait/notification completion by durable event ID.
Cancelled requests never revive from queued notifications.

:tasks opens a modal for the current session including earlier requests' tasks.
Show command, name, cwd, owner request, status, elapsed time and exit code. Open
live logs, cancel individual tasks, and filter completed tasks. Do not print env
secrets as default UI metadata. :stop is request-scoped, not list-wide cancellation.

## Subagent API

    agents.spawn({task, cwd?, provider?, model?, effort?}) -> {id}
    agents.list() -> child[]
    agents.status(id) -> child
    agents.send(id, text) -> {accepted, id}
    agents.wait(id, {timeout?}) -> childResultOrStatus
    agents.result(id) -> {id, status, result, error, usage}
    agents.cancel(id) -> child

No concurrency cap/launch queue. Only a main agent spawns children; no nesting.
No read-only/full flag or tool allowlist mode. Restrictions such as no file edits
are task instructions from the parent, not an OS or tool security sandbox.

Children receive explicit task/data/references and cwd project instructions, not
the parent's history/state. Inherit cwd/model/effort at launch unless supplied.
Use configured enabled providers; reject invalid model options without mutating
the parent's selection. The main agent manages Git worktrees through sh if useful;
meno does not create worktrees or copy uncommitted files automatically.

Child capabilities are ordinary tools except askUser, background tasks, nested
spawning, and main-session rename. Child state/TODO are independent and durable.
Preserve explicit role/provenance instructions. A child may read its own history
and send information to its parent, not control siblings or another parent's work.

Statuses: starting, running, blocked, completed, failed, cancelled, interrupted.
wait resolves on terminal state, timeout, or blocked (so a parent can answer a
blocker instead of deadlocking). result is repeatable; unfinished result is null.
Result text may describe checks; no changedFiles/checks schema fields. Errors and
usage are structured, with unknown prices distinct from zero.

## Parent/child communication

Child methods: agents.report(text), agents.block(reason). report sends bounded
progress through steering. block schedules a blocked transition after the current
tool group is durably complete and informs the parent; do not generate another
child model request until a message resumes it. If the same call fails, failure
wins over a pending block. Source code must not rely on a synchronous parent reply.

Parent send to running children is steering. Sending to completed/failed children
continues the same child session under the parent's current root request. Sending
to blocked children resumes them. Cancelled/interrupted children require an explicit
new launch rather than accidental automatic revival. Keep previous executions'
ownership immutable; the continuation receives a new execution generation.

Use bounded provider-request retries, not automatic whole-task restarts. Notify
the parent after failure with retained history. Sibling failures are independent.
If the parent responds without tools while children run, mark waiting; completion
steering resumes it to assess results and finish. Do not claim work is complete
merely because delegation completed.

## User interaction and accounting

:agents opens a modal showing only working children of the current session,
including starting/running/blocked. Select one, view its live conversation, and
send a message. Direct user messages use the same steering queue, with trusted
source metadata and a <user-message> wrapper. They do not reassign ownership.

No children in Sessions; no completed-child history viewer or added completion
manager. Persist child history internally for parent retrieval/continuation.
The parent can fetch its known child ID through bounded sessions.read.

Show per-child usage/cost and aggregate once in parent metrics. Separate direct
parent spend from delegated spend. Fork history does not count as new spending.
No child may ask the user a question; it reports a blocker to the parent instead.

## Acceptance

Test nonblocking launch, context isolation, inherited/explicit model/cwd, parallel
completion, message/block/resume races, wait returning blocked, retry exhaustion,
parent responsibility, cancellation, no nested/task/question bindings, active-only
UI, direct-user provenance, completed-child reuse, and exact cost aggregation.
