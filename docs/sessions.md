# Sessions, history and events

Several sessions run at once. Each has its own conversation, working directory,
model, effort, counters, pending question and half-typed message;
providers, MCP servers and settings are shared.

- `/resume` lists the last 200 stored sessions of all projects with live statuses: blinking green = running, orange `?` = waiting for an
  answer, red = last turn failed, blue = finished while you were elsewhere. The status bar
  counts the other sessions too (see [ui](ui.md#status-bar)).
  `Tab` cycles the filter all, running, asking, done; clicking a counter in the status bar opens the list already filtered.
- `/new` starts an empty session in the current directory; other sessions keep running. A session is stored (and listed) only when its first message is sent; an empty one is not kept. Its name appears as soon as that message is sent.
- `/fork` makes a new independent session from an earlier message (default: the
  latest). The boundary is widened to a whole tool-call group; the fork gets the
  history up to it, the summary that was valid *then* (never one that mentions later
  facts), model, effort, and directory as they were. No model call is
  made.
  Files and Git are not rewound; running tasks are not cloned; inherited
  history is not charged as new spending. A missing directory must be chosen with
  `/cd` first. Sessions created before the version journal existed can be forked as
  conversation only (meno says so).
- `/rename <name>` renames the session. Until then its name is the start of your first message; the agent cannot rename it.
- `/cd <path>` changes this session's directory (refused while it runs).
- `/compact` and the safety net summarize the old part of the context. The full
  original history stays in the database with stable message IDs; the active context
  only moves. The net runs before a request when the estimated input passes
  `compact_percent` (default 80) of a *known* context window, keeps at least
  `compact_keep` (8) messages, aims for a `compact_summary_tokens` (4000) summary, never
  compacts the same unchanged context twice, and fails with an actionable error when a
  request cannot fit.
- `/stop` / `Ctrl+C` cancel the active root request: its running turn, the bash calls
  still running in the foreground, and its pending events. Background tasks
  (`meno tasks start`, or a call moved to a task by your message) keep running;
  cancel one with `meno tasks cancel ID`. `/q` leaves meno; its runs and tasks end with it.

## Steering and events

A message you write while bash calls run moves the unfinished calls to background tasks and
the model answers at once. Each moved call gets one result with its task ID; the model reads
it with `meno tasks status|logs|wait|cancel`. Task-finished notices and other events never
interrupt a running call; they arrive after it ends.

One durable queue carries what happens while the agent works: your messages and
finished tasks. Each event has an ID, a source, an
owner (the root request it belongs to) and a state. Events are injected before the next
model request, never between a tool call and its result; during `meno ask` they wait; when
the session is idle an event starts a continuation. A cancelled request never resumes.
If an event of an old request arrives during a new one it joins the new one, and work
started because of it belongs to the new request. The model sees events in
`<event source=… from=… kind=…>` wrappers with escaped content. After a crash nothing is restarted. Unfinished
tasks are marked interrupted.

## Several windows

Each meno process (a window, `meno -p`, an ACP connection) runs its own engine. Stored sessions,
settings, providers and MCP definitions are shared through `MENO_HOME`; running turns and tasks are not:
they belong to the process that started them and end with it. A session has one owner: opening a
session that another process has open fails with a message naming the problem (the lock is dropped
by the system when that process dies, however it dies). Settings changed in one window reach the
others the next time they are read.

When the stream of a session breaks, the status bar says `offline · state may be
stale` and sending is held back with your draft kept; it clears on the next fresh
snapshot.
