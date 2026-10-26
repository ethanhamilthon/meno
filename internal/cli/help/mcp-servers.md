# mcp servers

Usage: meno mcp servers

List the configured servers, one per line: NAME (STATUS).

Status: running; starting (the handshake has not finished); stopped (enabled, not running);
disabled (switched off in mcp.json); error: MESSAGE (it could not start; the message says why).
--json prints [{"name", "status"}].

A server that is not listed is not in $MENO_HOME/mcp.json: see meno mcp add --help.
