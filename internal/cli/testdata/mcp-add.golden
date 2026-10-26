# mcp add

Usage: meno mcp add SERVER

Enable a server that is saved in mcp.json and switched off. The user is asked first; in a
session started with --no-ask it is refused (exit 3). It cannot create a server.

## Adding a new server

Edit $MENO_HOME/mcp.json (set in every call; ~/.meno by default). It is a JSON array:

  name       1-40 characters: a-z, 0-9, - and _
  transport  stdio (default), streamable-http or sse
  command    stdio: the program; args: its arguments as an array; cwd: working directory
  url        streamable-http and sse: the full http(s) address
  env        stdio: object of environment variables; headers: object of HTTP headers
  protocol   auto (default), legacy or modern
  enabled    true by default

  {"name": "exa", "transport": "streamable-http", "url": "https://mcp.exa.ai/mcp"}
  {"name": "files", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]}

The file can hold tokens: never print it whole. Steps:
 1. cp "$MENO_HOME/mcp.json" "$MENO_HOME/mcp.json.bak"
 2. Append one entry with jq or python3 (check with command -v) and keep the others.
 3. Check it is valid JSON: jq . "$MENO_HOME/mcp.json" >/dev/null
 4. meno mcp restart NAME, then meno mcp servers until NAME shows running.
 5. If it shows error or the list is empty, meno doctor says why; restore the .bak.
If the server needs a token, ask the user with meno ask and put it in env or headers.
