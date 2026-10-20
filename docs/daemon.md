# Daemon transport and configuration service

The daemon owns SQLite-backed sessions and agent runs. Closing a client does
not cancel a run. The current interactive TUI still uses the in-process app; the
client conversion is a later migration stage.
The client uses a startup lock, starts one daemon, and waits for its socket.

## Commands

- `meno daemon run` runs the local daemon in the foreground.
- `meno daemon start` starts it on demand.
- `meno daemon stop` asks it to stop.
- `meno daemon restart [--force]` stops and starts it. It refuses while
  sessions report active work unless `--force` is set.
- `meno daemon status` reports whether it responds.
- `meno daemon logs [-f]` prints the latest log lines. `-f` prints
  current file content; it does not yet follow new lines.

## Configuration routes

The daemon owns configuration reads and writes. The TUI still uses its current
in-process config path. Routes use JSON request and response objects under
`/v1`:

- `GET|PUT /settings` reads or replaces settings. The daemon reads
  `$MENO_HOME/settings.json` and falls back to the existing settings store
  when the file does not exist. It checks the file modification time on reads.
- `GET|POST /providers`, `GET|PUT|DELETE /providers/{name}`,
  `PUT /providers/{name}/enabled`, and
  `PUT /providers/{name}/models` manage API-key providers. Provider JSON and
  managed OAuth entries are read-only. API keys are not returned; `has_api_key`
  reports whether one is set. An update without a key keeps the current key.
  Set `clear_api_key` to remove it.
- `GET /login/services`, `POST /login/start`,
  `GET /login/status?state=…`, `POST /login/callback`,
  `GET /login/accounts`, `PUT|DELETE /login/accounts/{file}`,
  `GET /login/helper`, and `POST /login/restart` manage OAuth logins.
- `GET /skills` and `PUT /skills/{name}/enabled` list and toggle skills.
- `GET|POST /snippets`, `GET|PUT|DELETE /snippets/{name}`, and
  `PUT /snippets/{name}/rename` manage shared snippets.
- `GET|POST /mcp/servers`, `GET|PUT|DELETE /mcp/servers/{name}`, and
  `POST /mcp/servers/{name}/restart` manage MCP servers. mcp.json entries
  are read-only. Responses report starting, running, and error state.
- `GET /models?refresh=true` lists provider models and catalog pricing.
  Refresh also requests provider models and public catalog metadata.
- `GET /fs/search?root=…&q=…` performs the @ picker search.
- `GET /debug/requests?session=…` returns recent provider requests from an
  injected provider function.

Configuration writes and overlay file changes publish `settings_changed`,
`providers_changed`, and `mcp_changed` events through the global stream.
The service uses the existing SQLite stores and validators.

## Transport packages

- `internal/proto` defines JSON request, response, event, and error types.
- `internal/daemon` owns the HTTP server, SSE event hub, and lifecycle.
  Session operations use a `Backend` interface. `internal/engine` implements
  the backend, owns session runs, queues, tasks, subagents and MCP services, and
  publishes agent events through the session hub. Session resource routes use
  ephemeral bearer tokens. Configuration routes delegate to an injected handler.
- `internal/daemon/cfgsvc` owns daemon configuration resources.
- `internal/client` provides typed HTTP calls, daemon startup, SSE resume, and a
  view-state mirror.

The protocol number must match exactly. Session operations use a backend
interface. The command currently uses a stub backend, so session endpoints
return `not_implemented`. A client can fetch a snapshot, then resume the event
stream after its sequence number.
