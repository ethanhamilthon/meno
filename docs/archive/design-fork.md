# Design Decisions: Session Forking

Status: confirmed interview decisions; implementation specification in progress.

- Remove :reset. Existing :reset only resets the JS VM; the earlier interview
  description of it as creating a session was incorrect and is superseded.
- Keep :new as creation of an independent empty session with a new ID. Do not
  destroy the old session's history, state, or running work.
- Add :fork, selecting a user/assistant message, defaulting to the latest.
  Include the selected message and expand to a complete tool replay group when
  required. A message context action may use that message directly.
- Open the newly created session without automatic LLM execution. The next
  user message or explicit Continue starts it.
- Copy history through the boundary, applicable summary, cwd, model/effort,
  state, TODO, and acceptance criteria as they existed at that boundary.
- Keep versioned state/TODO mutations tied to message/step boundaries so old
  state can be reconstructed. Commits remain immediate and atomic.
- Copies are independent: child edits do not mutate the source session.
  Immutable image/artifact bytes may be shared with reference accounting.
- Do not clone or restart live tasks/subagents, transfer ownership, roll back
  project files/Git, or count inherited historical usage as new fork spending.
- Record source session ID and exact source boundary. Deliver a trusted fork
  system event on the next agent turn requiring session.rename again.
- If cwd is unavailable, require explicit directory selection rather than
  silently substituting another directory.
- For legacy boundaries without state/TODO versions, explicitly warn and offer
  a conversation-only fork with empty state/TODO. Do not fabricate old state or
  silently copy current state into an earlier boundary.
- Never use a summary containing facts after the fork point. Reconstruct from
  eligible original history when necessary; preserve protocol group boundaries.
