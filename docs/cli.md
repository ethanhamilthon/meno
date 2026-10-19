# Command-line commands

Use `meno read`, `meno edit`, `meno write`, or `meno search` for local file work. These commands do not start the daemon.

- `meno read FILE [START END] [--hash]` prints numbered lines. The optional hash is SHA-256 of the full file.
- `meno edit FILE OLD NEW` replaces one unique exact match. The JSON form reads a list of edits from stdin and requires `--expect-hash`. All edits succeed or none are written.
- `meno write FILE` reads stdin. It refuses to replace an existing file unless you pass `--overwrite`.
- `meno search PATTERN [DIR]` accepts `--regex`, `--glob G`, and `--limit N`.

Run `meno <group> --help` for command help. Use `--json` for structured output. Daemon-backed commands include `mcp`, `tasks`, `agents`, `ask`, `goal`, `web`, `view`, `models`, `daemon`, `sessions`, `send`, and `tail`. They require a daemon API adapter, which is not wired in this migration stage.

Exit codes are 0 for success, 1 for an operation error, 2 for invalid syntax, 3 for a mode restriction, and 124 when a wait times out.
