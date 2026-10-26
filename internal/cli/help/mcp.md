# mcp

Use the MCP servers the user configured. Your system prompt lists the running ones with
their tool names. Typical use:

    meno mcp tools SERVER            # one line per tool
    meno mcp tools SERVER TOOL       # full description and input schema of one tool
    meno mcp call SERVER TOOL '{"key": "value"}'

Commands: servers, tools SERVER [TOOL], call SERVER TOOL [ARGS], results ID, add SERVER,
remove SERVER, restart SERVER. Run meno mcp <command> --help for details.

- ARGS accepts one JSON object or key=value pairs. SERVER.TOOL is accepted for call.
- A server is running, starting, stopped, disabled or error: MESSAGE (see meno mcp servers).
- A result can be large: call prints it with a resultId; page through it with meno mcp results.
- To create a new server, edit $MENO_HOME/mcp.json: see meno mcp add --help.
- add and remove ask the user; restart does not.
