# tail

Usage: meno tail ID... [--children-of ID|me] [--events TYPE,...] [--from SEQ [--boot BOOT]]

Print the events of sessions as JSON lines while they run, until each session has ended. An idle session gives its turns. With several sessions, every line has a "session" field.

--events keeps the listed event types (text_delta, think_delta, call_start, call_end, turn_added, ask_open, ask_closed, usage, error, aborted, done; turn for an idle session).
--children-of follows the running sessions that ID started, and the ones that start meanwhile; it ends when none is running.
--from continues one session after an event number; with --boot (the "boot" of a line) the numbers belong to the same daemon run. For an idle session --from skips that many turns.
