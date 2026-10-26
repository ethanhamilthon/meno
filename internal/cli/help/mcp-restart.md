# mcp restart

Usage: meno mcp restart SERVER

Restart an enabled server, including one you just added to mcp.json. It runs without a
question, so an agent can recover a server that stopped answering. It returns at once, before
the server is up: run meno mcp servers until the status is running or error: MESSAGE.
A disabled server cannot be restarted (see meno mcp add --help).
