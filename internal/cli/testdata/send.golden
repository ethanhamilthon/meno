# send

Usage: meno send (-s ID | --new | --fork SESSION) [options] TEXT

Send a message and print the answer when the run is complete. There is no implicit session: choose one.

-s ID            write to an existing session
--new            create a session
--fork SESSION   create a session from the history of SESSION (up to --at N, default the last turn)

Options for --new and --fork: --cwd DIR, --title T, --system NAME (a file in system-prompts/; "default" is the built-in prompt), --no-ask.
--model PROVIDER/MODEL and --effort E set the model; with -s they change an idle session (its prompt cache restarts).

Waiting: the command runs until the run ends; to work in the background wrap it in `meno tasks start`. --timeout SECONDS stops waiting (exit 124, the run goes on). --detach does not wait.
If the session is already running the message is queued and the command returns at once.
A send that is interrupted (SIGINT, SIGTERM, `meno tasks cancel`) cancels the run of a session it created; --keep-running leaves it, --cancel-on-exit also cancels the run of an existing session.
If the agent opens a question (`meno ask`) the command prints what it has and exits 5; answer with `meno sessions answer`.

--json prints {sessionId, forkOf, status, text, question, usage, warnings}. Exit codes: 0 done, 1 error, 2 usage, 3 forbidden (worker limits), 5 waiting for an answer, 6 the run was stopped, 124 timeout.
