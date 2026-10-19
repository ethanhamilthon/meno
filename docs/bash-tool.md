# Bash execution package

The standalone internal/tools package provides primitives for the staged bash-tool migration. Integration supplies the session working directory, output directory, call ID, and complete caller environment. The executor does not inherit the daemon environment; include required host variables and MENO_* values in Env.

## Normal shell calls

Start starts a fresh /bin/sh -c process in a new session and process group. It has no terminal. Standard input is null. Standard output and error share one pipe. The environment is the supplied environment plus TERM=dumb, NO_COLOR=1, PAGER=cat, and GIT_PAGER=cat.

Call.Wait returns output followed by one status line: [exit N], [timed out after Ns], or [cancelled]. If the shell leaves processes in its group, the package kills them and adds the status suffix [killed N leftover processes; use meno tasks start for background work]. The timeout sends TERM to the group, then KILL after five seconds. A zero timeout disables the timer. Call.Pause and Call.Resume pause and resume the timer while an ask is open. Call.Cancel cancels the context and terminates the group.

The stream callback receives raw output deltas in pipe order. Returned output keeps the first 32 KiB and last 24 KiB. The full output is written to <OutputDir>/<CallID>.log, capped at 10 MiB with the newest bytes kept. DeleteSessionOutput removes a session output directory.

## Timeout setting and tool schema

ParseTimeoutSetting accepts a positive number of seconds, off, or model. An empty value selects 120 seconds. BashToolDefinition returns a plain Go struct with JSON tags and a provider-neutral schema. It adds the optional integer timeout argument only for the model setting. The package does not import provider or agent packages.

## Restricted commands

ParseRestrictedCommand accepts one meno <group> ... command. It checks the second word against a caller-provided allow-list and maps meno to the daemon's absolute executable path. Bare words use the spec's safe character set. Single-quoted and double-quoted parts can join a word; expansion and shell operators are rejected. RunRestricted executes the parsed argv directly. It never starts a shell.

## Heredoc display folding

FindHeredocBodies recognizes complete <<TAG, <<'TAG', and <<-TAG bodies. It returns source byte ranges for display code. It returns no ranges if it cannot parse the delimiter or find every closing tag. This conservative choice implements the open folding question: uncertain commands do not fold.

The 120-second default follows the proposed spec. It has not yet been checked against long builds. This package is not wired into the current agent loop; the daemon integration supplies that step.
