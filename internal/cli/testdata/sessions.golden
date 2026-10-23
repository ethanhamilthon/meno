# sessions

Usage: meno sessions [--cwd DIR] [--status idle|running|needs-you] [--spawned-by ID|me] [--title-prefix P] [--limit N]
       meno sessions show ID
       meno sessions cancel ID
       meno sessions answer ID QUESTION ANSWERS_JSON | --cancel

List sessions, newest first. `me` is the calling session. --json adds model, effort, spawnedBy, forkOf, system and turns.

show prints the state, the open question and the latest answer of one session.
cancel stops the run of a session (an error when there is nothing to stop).
answer replies to the open question of a session: ANSWERS_JSON is an object keyed by question id, for example '{"q1": "yes"}'; --cancel cancels the question.
