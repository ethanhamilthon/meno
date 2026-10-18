# tasks
tasks run shell commands in the background while you keep working.
- tasks.start(command, {cwd?, env?, name?, timeout?, notify?}) returns {id} as soon as the process exists. stdin is closed and there is no terminal. timeout is the process lifetime in seconds (none by default).
- tasks.list({includeFinished?}), tasks.status(id), tasks.cancel(id).
- tasks.logs(id, {cursor?, limit?, stream?}) returns {records: [{seq, stream, text}], nextCursor, lost, truncated}. The default is the newest 200 lines, at most 64 KB. Pass nextCursor to continue. lost counts output dropped by log rotation.
- tasks.wait(id, {timeout?}) returns the task with waitTimedOut. A timed-out wait never cancels the task.
Statuses: starting, running, succeeded, failed, cancelled, timed_out, interrupted. exitCode is null when there was no normal exit.
When a task finishes, you get an event with its id, status, and exit code (not its log). You do not get it if you started the task with notify: false or if you already saw the result through a successful tasks.wait.
Tasks live only while meno is open. They are cancelled when it exits. When the user stops your request, the tasks of that request are cancelled.
