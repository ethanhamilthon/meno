# mcpServers
mcpServers manages the MCP server configuration. Changes are saved, applied at once, and shown to the user. Every method is synchronous. Do not add, change, or remove servers unless the user asked.
- mcpServers.list() returns every configured server: {name, transport, command|url, enabled, status: "running"|"stopped"|"starting"|"failed", error?, tools: <count>, env?, headers?}. Secret values of env and headers are always shown as "***".
- mcpServers.add({name, transport?: "stdio"|"sse"|"streamable-http", command?, url?, env?: {K: "v"}, headers?: {K: "v"}, enabled?: true}) adds and starts a server. command is a shell-style command line for stdio. url is for sse and streamable-http.
- mcpServers.update(name, patch) changes any of those fields and restarts the server. env and headers are replaced as a whole. enabled: false stops the server.
- mcpServers.remove(name) and mcpServers.restart(name).
add, update, and restart wait until the server is up. They return {ok, error?, tools: [{name, description}]} and do not throw. After success, the tools are callable at once as mcp.<server>.<tool>(). Read the mcp global again instead of keeping a reference to it.
Servers defined in mcp.json are read only. You cannot change or remove them.
