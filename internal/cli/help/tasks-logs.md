# tasks logs

Usage: meno tasks logs ID [--from N --limit N]

Print the output of a task, stdout and stderr together. Without --from you get the newest
--limit records (default 200). With --from N you get the records after number N, oldest first,
so you can read a running task piece by piece. --json prints {"records": [{"seq", "stream",
"text"}], "nextCursor", "lost", "truncated"}: pass nextCursor as --from to read on. Logs are
bounded: "lost" counts old records rotated away and "truncated" says the page was cut.
