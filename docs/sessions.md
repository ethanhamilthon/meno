# Sessions, history and events

Several sessions run at once. Each has its own conversation, working directory,
model, effort, counters, pending question and half-typed message;
providers, MCP servers and settings are shared.

- `:sessions` lists the last 200 sessions of all projects with live statuses: blinking green = running, orange `?` = waiting for an
  answer, red = last turn failed, blue = finished while you were elsewhere. The status bar
  counts the other sessions too (see [ui](ui.md#status-bar)).
- `:new` starts an empty session in the current directory; other sessions keep running.
- `:fork` makes a new independent session from an earlier message (default: the
  latest). The boundary is widened to a whole tool-call group; the fork gets the
  history up to it, the summary that was valid *then* (never one that mentions later
  facts), model, effort, and directory as they were. No model call is
  made.
  Files and Git are not rewound; running tasks are not cloned; inherited
  history is not charged as new spending. A missing directory must be chosen with
  `:cd` first. Sessions created before the version journal existed can be forked as
  conversation only (meno says so).
- `:rename <name>` renames the session. Until then its name is the start of your first message; the agent cannot rename it.
- `:cd <path>` changes this session's directory (refused while it runs).
- `:compact` and the safety net summarize the old part of the context. The full
  original history stays in the database with stable message IDs; the active context
  only moves. The net runs before a request when the estimated input passes
  `compact_percent` (default 80) of a *known* context window, keeps at least
  `compact_keep` (8) messages, aims for a `compact_summary_tokens` (4000) summary, never
  compacts the same unchanged context twice, and fails with an actionable error when a
  request cannot fit.
- `:stop` / `Ctrl+C` cancel the active root request: its running turn, the tasks
  it started, and its pending events. Work from earlier requests keeps
  running. `:q` refuses while anything runs; `:q!` cancels everything.

## Steering and events

One durable queue carries what happens while the agent works: your messages and
finished tasks. Each event has an ID, a source, an
owner (the root request it belongs to) and a state. Events are injected before the next
model request, never between a tool call and its result; during `meno ask` they wait; when
the session is idle an event starts a continuation. A cancelled request never resumes.
If an event of an old request arrives during a new one it joins the new one, and work
started because of it belongs to the new request. The model sees events in
`<event source=… from=… kind=…>` wrappers with escaped content. After a crash nothing is restarted. Unfinished
tasks are marked interrupted.
