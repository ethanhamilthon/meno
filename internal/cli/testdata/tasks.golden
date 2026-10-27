# tasks

Run a command in the background so your call can end: a dev server, a long build, a watcher.
A task outlives the call and the window; its output is kept in bounded rotating logs.

Commands: start CMD [--cwd DIR] [--env K=V] [--name NAME] [--no-notify], list [--all],
status ID, logs ID [--from N] [--limit N], wait ID [--timeout SECONDS], cancel ID.

Status: starting, running, succeeded, failed, cancelled, timed_out, interrupted (meno
stopped while it ran; it is never restarted). --json adds exitCode.

wait returns exit code 124 when its timeout expires; a timeout does not cancel the task.
When a task ends you get a notice, unless it was started with --no-notify.
