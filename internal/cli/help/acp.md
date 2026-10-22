# meno acp

Serve Meno as an Agent Client Protocol (ACP) agent on stdin and stdout.

Usage: `meno acp`. Any ACP client can launch this command as its agent. Takes no
arguments; protocol messages use stdout and diagnostics use stderr.

Supports chat, tool cards, cancellation, creating, listing and loading sessions,
skills and slash commands, model and effort selection, usage and titles, exposed
thinking, image and text-resource prompts. Sessions are ordinary Meno sessions
and stay visible in the terminal app.

Questions that Meno asks are shown as chat messages. Reply with
`/answer <question-id> <answers-json>` or `/answer <question-id> --cancel`; the
paused run then continues. Closing the connection leaves an open question and its
run waiting; closing it during a normal run stops that run. The daemon keeps running.

Not supported: MCP servers injected by the client (configure them with
`meno mcp`), per-command permission prompts, audio and PDF content.
